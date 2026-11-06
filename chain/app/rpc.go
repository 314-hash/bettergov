package app

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bettergov/chain/x/bank"
	"github.com/bettergov/chain/x/evm"
	"github.com/bettergov/chain/x/gov"
	"github.com/bettergov/chain/x/staking"
)

type JSONRPCRequest struct {
	JSONRPC string            `json:"jsonrpc"`
	Method  string            `json:"method"`
	Params  []json.RawMessage `json:"params"`
	ID      interface{}       `json:"id"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
	ID      interface{}   `json:"id"`
}

// NewPublicMux returns an HTTP router serving Ethereum JSON-RPC 2.0 and Cosmos/BetterGov REST endpoints.
func (a *BetterGovApp) NewPublicMux() *http.ServeMux {
	mux := http.NewServeMux()

	// Ethereum JSON-RPC 2.0 Endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/" {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"service":      "BETTERGOV Layer 1 Public RPC",
				"chain_id":     a.ChainID,
				"evm_chain_id": a.EVMChainID,
				"height":       a.Height,
				"app_hash":     a.LastAppHash,
			})
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, JSONRPCResponse{
				JSONRPC: "2.0",
				Error:   &JSONRPCError{Code: -32700, Message: "parse error"},
				ID:      nil,
			})
			return
		}
		resp := a.HandleJSONRPC(req)
		writeJSON(w, http.StatusOK, resp)
	})

	// Cosmos / BetterGov REST API Endpoints
	mux.HandleFunc("/bettergov/network/status", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		a.mu.RLock()
		defer a.mu.RUnlock()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"chain_id":         a.ChainID,
			"evm_chain_id":     a.EVMChainID,
			"latest_height":    a.Height,
			"latest_app_hash":  a.LastAppHash,
			"total_supply":     a.Bank.GetTotalSupply().String(),
			"total_burned":     a.Bank.GetTotalBurned().String(),
			"total_bonded":     a.Staking.GetTotalBondedTokens().String(),
			"treasury_balance": a.Bank.GetBalance(bank.PoolTreasury).String(),
			"active_validators": len(a.Staking.GetValidatorUpdates()),
			"economic_params":  a.Mint.GetParams(),
		})
	})

	mux.HandleFunc("/cosmos/staking/v1beta1/validators", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"validators":           a.Staking.GetAllValidators(),
			"params":               a.Staking.GetParams(),
			"unlimited_validators": true,
			"available_roles":      staking.AvailableValidatorRoles(),
		})
	})

	// Live Validator Registration Endpoint (Unlimited Validator Set + BETTERGOV Role)
	mux.HandleFunc("/cosmos/staking/v1beta1/create-validator", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			OperatorAddress       string `json:"operator_address"`
			ConsensusPubKeyHex    string `json:"consensus_pubkey_hex"`
			Moniker               string `json:"moniker"`
			Role                  string `json:"role"`
			Website               string `json:"website"`
			SecurityContact       string `json:"security_contact"`
			Details               string `json:"details"`
			SelfBondBGOV          uint64 `json:"self_bond_bgov"`
			SelfBondAbgov         string `json:"self_bond_abgov"`
			MinSelfDelegationBGOV uint64 `json:"min_self_delegation_bgov"`
			CommissionRateBps     uint64 `json:"commission_rate_bps"`
			CommissionMaxRateBps  uint64 `json:"commission_max_rate_bps"`
			CommissionChangeBps   uint64 `json:"commission_max_change_bps"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		var selfBond *big.Int
		if strings.TrimSpace(req.SelfBondAbgov) != "" {
			parsed, err := bank.ParseAbgov(req.SelfBondAbgov)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid self_bond_abgov"})
				return
			}
			selfBond = parsed
		} else {
			if req.SelfBondBGOV == 0 {
				req.SelfBondBGOV = 100_000
			}
			selfBond = bank.BGOVToAbgov(req.SelfBondBGOV)
		}

		if req.MinSelfDelegationBGOV == 0 {
			req.MinSelfDelegationBGOV = 50_000
		}
		minSelf := bank.BGOVToAbgov(req.MinSelfDelegationBGOV)
		if selfBond.Cmp(minSelf) < 0 {
			minSelf = new(big.Int).Set(selfBond)
		}

		if req.CommissionRateBps == 0 {
			req.CommissionRateBps = 500
		}
		if req.CommissionMaxRateBps == 0 {
			req.CommissionMaxRateBps = 2000
		}
		if req.CommissionChangeBps == 0 {
			req.CommissionChangeBps = 100
		}

		// On local devnet, auto-fund the operator address if it lacks sufficient balance for self-bonding
		curBal := a.Bank.GetBalance(req.OperatorAddress)
		if curBal.Cmp(selfBond) < 0 && strings.Contains(a.ChainID, "devnet") {
			diff := new(big.Int).Sub(selfBond, curBal)
			_ = a.Bank.MintTokens(req.OperatorAddress, diff)
		}

		now := time.Now().UTC()
		val, err := a.Staking.CreateValidatorWithRole(
			req.OperatorAddress,
			req.ConsensusPubKeyHex,
			req.Moniker,
			req.Role,
			req.Website,
			req.SecurityContact,
			req.Details,
			selfBond,
			minSelf,
			staking.CommissionRates{
				RateBps:          req.CommissionRateBps,
				MaxRateBps:       req.CommissionMaxRateBps,
				MaxChangeRateBps: req.CommissionChangeBps,
			},
			now,
		)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		a.Slashing.EnsureSigningInfo(val.ConsensusAddress, a.Height+1)
		hdr, _ := a.FinalizeBlock(now, nil, nil)

		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"status":            "VALIDATOR_REGISTERED_AND_BONDED",
			"validator":         val,
			"active_validators": len(a.Staking.GetValidatorUpdates()),
			"unlimited_set":     true,
			"block_height":      hdr.Height,
			"app_hash":          hdr.AppHash,
		})
	})

	// Live Delegation Endpoint
	mux.HandleFunc("/cosmos/staking/v1beta1/delegate", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			DelegatorAddress string `json:"delegator_address"`
			ValidatorAddress string `json:"validator_address"`
			AmountBGOV       uint64 `json:"amount_bgov"`
			AmountAbgov      string `json:"amount_abgov"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		var amt *big.Int
		if strings.TrimSpace(req.AmountAbgov) != "" {
			parsed, err := bank.ParseAbgov(req.AmountAbgov)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid amount_abgov"})
				return
			}
			amt = parsed
		} else {
			amt = bank.BGOVToAbgov(req.AmountBGOV)
		}

		shares, err := a.Staking.Delegate(req.DelegatorAddress, req.ValidatorAddress, amt)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		hdr, _ := a.FinalizeBlock(time.Now().UTC(), nil, nil)
		val, _ := a.Staking.GetValidator(req.ValidatorAddress)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":        "DELEGATION_BONDED",
			"shares_issued": shares.String(),
			"validator":     val,
			"block_height":  hdr.Height,
			"app_hash":      hdr.AppHash,
		})
	})

	// Live Validator Unjail Endpoint
	mux.HandleFunc("/cosmos/staking/v1beta1/unjail", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			OperatorAddress string `json:"operator_address"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := a.Slashing.Unjail(req.OperatorAddress, time.Now().UTC()); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		hdr, _ := a.FinalizeBlock(time.Now().UTC(), nil, nil)
		val, _ := a.Staking.GetValidator(req.OperatorAddress)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":       "VALIDATOR_UNJAILED",
			"validator":    val,
			"block_height": hdr.Height,
		})
	})

	mux.HandleFunc("/cosmos/gov/v1/proposals", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"proposals": a.Gov.GetAllProposals(),
			"params":    a.Gov.GetParams(),
		})
	})

	// Live Governance Proposal Submission Endpoint
	mux.HandleFunc("/cosmos/gov/v1/submit-proposal", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Proposer           string `json:"proposer"`
			Title              string `json:"title"`
			Summary            string `json:"summary"`
			MetadataURI        string `json:"metadata_uri"`
			ProposalType       string `json:"proposal_type"`
			InitialDepositBGOV uint64 `json:"initial_deposit_bgov"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if req.Proposer == "" {
			req.Proposer = "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"
		}
		if req.InitialDepositBGOV == 0 {
			req.InitialDepositBGOV = 1000
		}
		pType := gov.ProposalTypeText
		if req.ProposalType != "" {
			pType = gov.ProposalType(req.ProposalType)
		}
		now := time.Now().UTC()
		prop, err := a.Gov.SubmitProposal(
			req.Proposer,
			req.Title,
			req.Summary,
			req.MetadataURI,
			pType,
			bank.BGOVToAbgov(req.InitialDepositBGOV),
			nil, nil, nil,
			now,
		)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		hdr, _ := a.FinalizeBlock(now, nil, nil)
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"status":       "PROPOSAL_SUBMITTED",
			"proposal":     prop,
			"block_height": hdr.Height,
		})
	})

	mux.HandleFunc("/bettergov/transparency/v1/records", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"disclaimer":   "ON-CHAIN HASH PROVES TIMESTAMP AND DATA INTEGRITY ONLY.",
			"documents":    a.Transparency.GetAllDocuments(),
			"projects":     a.Transparency.GetAllProjects(),
			"procurements": a.Transparency.GetAllProcurements(),
			"budgets":      a.Transparency.GetAllBudgets(),
		})
	})

	return mux
}

// NewPrivateAdminMux returns the private validator & node administration router, enforcing strict loopback IP checks.
func (a *BetterGovApp) NewPrivateAdminMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/admin/snapshot", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, a.ExportSnapshot())
	})
	mux.HandleFunc("/admin/validator-set", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"validator_updates": a.Staking.GetValidatorUpdates(),
			"slash_events":      a.Slashing.GetSlashEvents(),
		})
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			http.Error(w, "FORBIDDEN: administrative validator RPC is restricted to loopback (127.0.0.1 / ::1)", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (a *BetterGovApp) HandleJSONRPC(req JSONRPCRequest) JSONRPCResponse {
	switch req.Method {
	case "web3_clientVersion":
		return JSONRPCResponse{JSONRPC: "2.0", Result: "BetterGovD/v1.0.0/cosmos-evm-go", ID: req.ID}

	case "eth_chainId":
		return JSONRPCResponse{JSONRPC: "2.0", Result: fmt.Sprintf("0x%x", a.EVM.ChainID()), ID: req.ID}

	case "net_version":
		return JSONRPCResponse{JSONRPC: "2.0", Result: strconv.FormatUint(a.EVM.ChainID(), 10), ID: req.ID}

	case "eth_blockNumber":
		a.mu.RLock()
		h := a.Height
		a.mu.RUnlock()
		return JSONRPCResponse{JSONRPC: "2.0", Result: fmt.Sprintf("0x%x", h), ID: req.ID}

	case "eth_gasPrice":
		return JSONRPCResponse{JSONRPC: "2.0", Result: "0x" + a.EVM.MinGasPrice().Text(16), ID: req.ID}

	case "eth_getBalance":
		if len(req.Params) < 1 {
			return rpcErr(req.ID, -32602, "missing address parameter")
		}
		var addr string
		_ = json.Unmarshal(req.Params[0], &addr)
		bal := a.Bank.GetBalance(addr)
		return JSONRPCResponse{JSONRPC: "2.0", Result: "0x" + bal.Text(16), ID: req.ID}

	case "eth_getTransactionCount":
		if len(req.Params) < 1 {
			return rpcErr(req.ID, -32602, "missing address parameter")
		}
		var addr string
		_ = json.Unmarshal(req.Params[0], &addr)
		nonce := a.EVM.GetNonce(addr)
		return JSONRPCResponse{JSONRPC: "2.0", Result: fmt.Sprintf("0x%x", nonce), ID: req.ID}

	case "eth_getCode":
		if len(req.Params) < 1 {
			return rpcErr(req.ID, -32602, "missing address parameter")
		}
		var addr string
		_ = json.Unmarshal(req.Params[0], &addr)
		return JSONRPCResponse{JSONRPC: "2.0", Result: a.EVM.GetCode(addr), ID: req.ID}

	case "eth_estimateGas":
		if len(req.Params) < 1 {
			return JSONRPCResponse{JSONRPC: "2.0", Result: fmt.Sprintf("0x%x", evm.BaseTxGas), ID: req.ID}
		}
		var callObj struct {
			To   string `json:"to"`
			Data string `json:"data"`
		}
		_ = json.Unmarshal(req.Params[0], &callObj)
		est := evm.IntrinsicGas(callObj.Data, strings.TrimSpace(callObj.To) == "")
		if len(strings.TrimPrefix(callObj.Data, "0x")) > 0 && strings.TrimSpace(callObj.To) != "" {
			est += evm.SStoreGas
		}
		return JSONRPCResponse{JSONRPC: "2.0", Result: fmt.Sprintf("0x%x", est), ID: req.ID}

	case "eth_call":
		if len(req.Params) < 1 {
			return rpcErr(req.ID, -32602, "missing call object")
		}
		var callObj struct {
			To   string `json:"to"`
			Data string `json:"data"`
		}
		_ = json.Unmarshal(req.Params[0], &callObj)
		res, err := a.EVM.EthCall(callObj.To, callObj.Data)
		if err != nil {
			return rpcErr(req.ID, -32000, err.Error())
		}
		return JSONRPCResponse{JSONRPC: "2.0", Result: res, ID: req.ID}

	case "eth_sendTransaction":
		if len(req.Params) < 1 {
			return rpcErr(req.ID, -32602, "missing transaction object")
		}
		var txIn struct {
			From     string `json:"from"`
			To       string `json:"to"`
			Value    string `json:"value"`
			Data     string `json:"data"`
			Gas      string `json:"gas"`
			GasPrice string `json:"gasPrice"`
		}
		if err := json.Unmarshal(req.Params[0], &txIn); err != nil {
			return rpcErr(req.ID, -32602, err.Error())
		}
		val := parseHexOrDecBig(txIn.Value)
		gp := parseHexOrDecBig(txIn.GasPrice)
		if gp.Sign() <= 0 {
			gp = a.EVM.MinGasPrice()
		}
		gasLim := uint64(250000)
		if txIn.Gas != "" {
			if parsedG := parseHexOrDecBig(txIn.Gas).Uint64(); parsedG > 0 {
				gasLim = parsedG
			}
		}
		tx := &evm.EVMTransaction{
			ChainID:  a.EVM.ChainID(),
			Nonce:    a.EVM.GetNonce(txIn.From),
			From:     txIn.From,
			To:       txIn.To,
			Value:    val,
			DataHex:  txIn.Data,
			GasLimit: gasLim,
			GasPrice: gp,
		}
		_, err := a.FinalizeBlock(time.Now().UTC(), []*evm.EVMTransaction{tx}, nil)
		if err != nil {
			return rpcErr(req.ID, -32000, err.Error())
		}
		return JSONRPCResponse{JSONRPC: "2.0", Result: tx.Hash, ID: req.ID}

	case "eth_getTransactionByHash":
		if len(req.Params) < 1 {
			return rpcErr(req.ID, -32602, "missing tx hash")
		}
		var h string
		_ = json.Unmarshal(req.Params[0], &h)
		tx, ok := a.EVM.GetTransaction(h)
		if !ok {
			return JSONRPCResponse{JSONRPC: "2.0", Result: nil, ID: req.ID}
		}
		return JSONRPCResponse{JSONRPC: "2.0", Result: tx, ID: req.ID}

	case "eth_getTransactionReceipt":
		if len(req.Params) < 1 {
			return rpcErr(req.ID, -32602, "missing tx hash")
		}
		var h string
		_ = json.Unmarshal(req.Params[0], &h)
		rc, ok := a.EVM.GetReceipt(h)
		if !ok {
			return JSONRPCResponse{JSONRPC: "2.0", Result: nil, ID: req.ID}
		}
		return JSONRPCResponse{JSONRPC: "2.0", Result: rc, ID: req.ID}

	case "eth_getBlockByNumber":
		a.mu.RLock()
		defer a.mu.RUnlock()
		if len(a.Blocks) == 0 {
			return JSONRPCResponse{JSONRPC: "2.0", Result: nil, ID: req.ID}
		}
		latest := a.Blocks[len(a.Blocks)-1]
		return JSONRPCResponse{JSONRPC: "2.0", Result: latest, ID: req.ID}

	default:
		return rpcErr(req.ID, -32601, fmt.Sprintf("method %s not found", req.Method))
	}
}

func rpcErr(id interface{}, code int, msg string) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: "2.0",
		Error:   &JSONRPCError{Code: code, Message: msg},
		ID:      id,
	}
}

func parseHexOrDecBig(s string) *big.Int {
	s = strings.TrimSpace(s)
	if s == "" {
		return big.NewInt(0)
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		v, ok := new(big.Int).SetString(s[2:], 16)
		if ok {
			return v
		}
	}
	v, ok := new(big.Int).SetString(s, 10)
	if ok {
		return v
	}
	return big.NewInt(0)
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

