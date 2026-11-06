package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bettergov/chain/app"
	"github.com/bettergov/chain/x/staking"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcmd := os.Args[1]
	switch subcmd {
	case "init":
		initCmd := flag.NewFlagSet("init", flag.ExitOnError)
		outPath := initCmd.String("output", "genesis.json", "Path to write genesis file")
		_ = initCmd.Parse(os.Args[2:])

		gen := app.DefaultDevnetGenesis()
		raw, err := json.MarshalIndent(gen, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal genesis: %v", err)
		}
		if err := os.WriteFile(*outPath, raw, 0600); err != nil {
			log.Fatalf("failed to write genesis file: %v", err)
		}
		fmt.Printf("[bettergovd] initialized genesis for chain %s (EVM ID %d, Unlimited Validators=true) -> %s\n",
			gen.ChainID, gen.EVMChainID, *outPath)

	case "genesis-validate":
		valCmd := flag.NewFlagSet("genesis-validate", flag.ExitOnError)
		genPath := valCmd.String("file", "genesis.json", "Path to genesis.json")
		_ = valCmd.Parse(os.Args[2:])

		raw, err := os.ReadFile(*genPath)
		if err != nil {
			log.Fatalf("failed to read %s: %v", *genPath, err)
		}
		var gen app.GenesisState
		if err := json.Unmarshal(raw, &gen); err != nil {
			log.Fatalf("invalid genesis JSON: %v", err)
		}
		chainApp, err := app.NewBetterGovApp(gen)
		if err != nil {
			log.Fatalf("genesis state validation failed: %v", err)
		}
		fmt.Printf("[bettergovd] genesis VALID: chain_id=%s evm_chain_id=%d validators=%d unlimited_validators=true initial_app_hash=%s\n",
			chainApp.ChainID, chainApp.EVMChainID, len(chainApp.Staking.GetValidatorUpdates()), chainApp.LastAppHash)

	case "start":
		startCmd := flag.NewFlagSet("start", flag.ExitOnError)
		rpcBind := startCmd.String("rpc-bind", "127.0.0.1:8545", "Public JSON-RPC and REST bind address")
		adminBind := startCmd.String("admin-bind", "127.0.0.1:26657", "Private Validator Admin RPC bind address (loopback only)")
		blockInterval := startCmd.Duration("block-time", 3*time.Second, "Consensus block production interval")
		_ = startCmd.Parse(os.Args[2:])

		gen := app.DefaultDevnetGenesis()
		chainApp, err := app.NewBetterGovApp(gen)
		if err != nil {
			log.Fatalf("failed to initialize BetterGovApp: %v", err)
		}

		// Produce initial block 1
		_, _ = chainApp.FinalizeBlock(time.Now().UTC(), nil, nil)

		// Background block producer ticker
		go func() {
			ticker := time.NewTicker(*blockInterval)
			defer ticker.Stop()
			for t := range ticker.C {
				_, _ = chainApp.FinalizeBlock(t.UTC(), nil, nil)
			}
		}()

		// Start Private Admin RPC (loopback only)
		go func() {
			log.Printf("[bettergovd] Private Validator Admin RPC listening on http://%s (loopback-restricted)", *adminBind)
			if err := http.ListenAndServe(*adminBind, chainApp.NewPrivateAdminMux()); err != nil {
				log.Fatalf("admin RPC failed: %v", err)
			}
		}()

		log.Printf("[bettergovd] Public JSON-RPC & Cosmos REST listening on http://%s (ChainID=%s, EVM=%d, MaxValidators=UNLIMITED)",
			*rpcBind, chainApp.ChainID, chainApp.EVMChainID)
		if err := http.ListenAndServe(*rpcBind, chainApp.NewPublicMux()); err != nil {
			log.Fatalf("public RPC failed: %v", err)
		}

	case "status":
		gen := app.DefaultDevnetGenesis()
		chainApp, err := app.NewBetterGovApp(gen)
		if err != nil {
			log.Fatalf("failed to initialize app: %v", err)
		}
		_, _ = chainApp.FinalizeBlock(time.Now().UTC(), nil, nil)
		status := map[string]interface{}{
			"node":                 "bettergovd/v1.0.0",
			"chain_id":             chainApp.ChainID,
			"evm_chain_id":         chainApp.EVMChainID,
			"height":               chainApp.Height,
			"app_hash":             chainApp.LastAppHash,
			"active_validators":    len(chainApp.Staking.GetValidatorUpdates()),
			"unlimited_validators": true,
			"available_roles":      staking.AvailableValidatorRoles(),
			"total_bonded":         chainApp.Staking.GetTotalBondedTokens().String(),
		}
		out, _ := json.MarshalIndent(status, "", "  ")
		fmt.Println(string(out))

	case "validator":
		handleValidatorCLI(os.Args[2:])

	case "tx":
		if len(os.Args) >= 3 && os.Args[2] == "staking" {
			handleValidatorCLI(os.Args[3:])
		} else {
			printUsage()
			os.Exit(1)
		}

	default:
		printUsage()
		os.Exit(1)
	}
}

func handleValidatorCLI(args []string) {
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}
	action := args[0]
	switch action {
	case "roles":
		out, _ := json.MarshalIndent(map[string]interface{}{
			"max_validators":       "UNLIMITED (0)",
			"unlimited_validators": true,
			"roles":                staking.AvailableValidatorRoles(),
		}, "", "  ")
		fmt.Println(string(out))

	case "keygen":
		kgCmd := flag.NewFlagSet("keygen", flag.ExitOnError)
		role := kgCmd.String("role", staking.RoleConsensusGuardian, "BETTERGOV institutional role for this validator")
		_ = kgCmd.Parse(args[1:])

		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			log.Fatalf("ed25519 keygen failed: %v", err)
		}
		pubHex := hex.EncodeToString(pub)
		privHex := hex.EncodeToString(priv.Seed())
		consAddr, _ := staking.DeriveConsensusAddress(pubHex)

		var opBytes [20]byte
		_, _ = rand.Read(opBytes[:])
		opAddr := "0x" + hex.EncodeToString(opBytes[:])

		out, _ := json.MarshalIndent(map[string]interface{}{
			"operator_address":             opAddr,
			"consensus_pubkey_hex":         pubHex,
			"consensus_privkey_hex_SECRET": privHex,
			"consensus_address":            consAddr,
			"assigned_role":                staking.NormalizeValidatorRole(*role),
			"validator_set_capacity":       "UNLIMITED",
			"warning":                      "Store consensus_privkey_hex_SECRET offline. Never commit private keys to git.",
		}, "", "  ")
		fmt.Println(string(out))

	case "create", "create-validator":
		crCmd := flag.NewFlagSet("create", flag.ExitOnError)
		rpcURL := crCmd.String("rpc", "http://127.0.0.1:8545", "BETTERGOV node RPC URL")
		opAddr := crCmd.String("operator", "", "Validator operator 0x address (auto-generated if omitted)")
		pubKey := crCmd.String("pubkey", "", "32-byte Ed25519 consensus public key hex (auto-generated if omitted)")
		moniker := crCmd.String("moniker", "Civic-Validator-Node", "Validator moniker / display name")
		role := crCmd.String("role", staking.RoleConsensusGuardian, "BETTERGOV Role (CONSENSUS_GUARDIAN, PUBLIC_AUDITOR_NODE, PROCUREMENT_WATCHDOG, TREASURY_SENTINEL, REGIONAL_CIVIC_RELAY, POST_QUANTUM_NOTARY)")
		website := crCmd.String("website", "https://validator.bettergov.org", "Validator website")
		secContact := crCmd.String("security-contact", "sec-ops-validator", "Security contact identifier")
		details := crCmd.String("details", "Independent BETTERGOV Civic Validator", "Validator description")
		selfBond := crCmd.Uint64("self-bond", 100000, "Self-bond amount in whole BGOV (min 50000)")
		minSelf := crCmd.Uint64("min-self-delegation", 50000, "Minimum self-delegation in whole BGOV")
		commRate := crCmd.Uint64("commission-bps", 500, "Initial commission rate in bps (500 = 5.00%)")
		_ = crCmd.Parse(args[1:])

		if *opAddr == "" {
			var opBytes [20]byte
			_, _ = rand.Read(opBytes[:])
			*opAddr = "0x" + hex.EncodeToString(opBytes[:])
		}
		if *pubKey == "" {
			pub, _, _ := ed25519.GenerateKey(rand.Reader)
			*pubKey = hex.EncodeToString(pub)
		}

		payload := map[string]interface{}{
			"operator_address":         *opAddr,
			"consensus_pubkey_hex":     *pubKey,
			"moniker":                  *moniker,
			"role":                     staking.NormalizeValidatorRole(*role),
			"website":                  *website,
			"security_contact":         *secContact,
			"details":                  *details,
			"self_bond_bgov":           *selfBond,
			"min_self_delegation_bgov": *minSelf,
			"commission_rate_bps":      *commRate,
			"commission_max_rate_bps":  2000,
			"commission_max_change_bps": 100,
		}
		postToNode(*rpcURL+"/cosmos/staking/v1beta1/create-validator", payload)

	case "delegate":
		delCmd := flag.NewFlagSet("delegate", flag.ExitOnError)
		rpcURL := delCmd.String("rpc", "http://127.0.0.1:8545", "BETTERGOV node RPC URL")
		delegator := delCmd.String("delegator", "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266", "Delegator 0x address")
		validator := delCmd.String("validator", "0x70997970c51812dc3a010c7d01b50e0d17dc79c8", "Validator operator 0x address")
		amount := delCmd.Uint64("amount", 25000, "Amount of BGOV to delegate")
		_ = delCmd.Parse(args[1:])

		payload := map[string]interface{}{
			"delegator_address": *delegator,
			"validator_address": *validator,
			"amount_bgov":       *amount,
		}
		postToNode(*rpcURL+"/cosmos/staking/v1beta1/delegate", payload)

	case "unjail":
		ujCmd := flag.NewFlagSet("unjail", flag.ExitOnError)
		rpcURL := ujCmd.String("rpc", "http://127.0.0.1:8545", "BETTERGOV node RPC URL")
		opAddr := ujCmd.String("operator", "", "Jailed validator operator 0x address")
		_ = ujCmd.Parse(args[1:])
		if *opAddr == "" {
			log.Fatalf("--operator is required for unjail")
		}
		postToNode(*rpcURL+"/cosmos/staking/v1beta1/unjail", map[string]interface{}{
			"operator_address": *opAddr,
		})

	default:
		printUsage()
		os.Exit(1)
	}
}

func postToNode(endpoint string, payload map[string]interface{}) {
	body, _ := json.Marshal(payload)
	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Fatalf("failed to connect to BETTERGOV node at %s: %v", endpoint, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err == nil {
		fmt.Println(pretty.String())
	} else {
		fmt.Println(string(raw))
	}
	if resp.StatusCode >= 400 {
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`BETTERGOV Layer 1 Node & Validator Daemon (bettergovd)
Usage:
  bettergovd init [--output genesis.json]                                 Generate default genesis configuration
  bettergovd genesis-validate [--file genesis.json]                       Validate genesis state and compute initial AppHash
  bettergovd start [--rpc-bind 127.0.0.1:8545]                            Start PoS consensus, EVM JSON-RPC, REST, and Private Admin RPC
  bettergovd status                                                       Print node consensus, unlimited validator set, and role status
  bettergovd validator roles                                              List official BETTERGOV validator roles (MaxValidators = UNLIMITED)
  bettergovd validator keygen [--role PUBLIC_AUDITOR_NODE]                Generate Ed25519 consensus keypair & operator address
  bettergovd validator create --moniker <name> --role <ROLE>              Register a new validator on the live unlimited validator set
  bettergovd validator delegate --validator <0x...> --amount <BGOV>       Delegate BGOV to a validator
  bettergovd validator unjail --operator <0x...>                          Unjail a temporarily jailed validator
  bettergovd tx staking <keygen|create|delegate|unjail|roles> [flags]     Cosmos-style alias for validator staking commands`)
}
