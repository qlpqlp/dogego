// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"
)

// rpcMethodHelp is Core-style one-line (or short) help for help("command").
// Full RPC semantics follow Dogecoin Core documentation where applicable.
var rpcMethodHelp = map[string]string{
	"addmultisigaddress":           "Add a P2SH multisig address as watch-only; spend via sendtoaddress when cosigner keys are importprivkey'd (extra_privkeys_hex).",
	"addnode":                      "Adds, removes, or dials a peer (add / remove / onetry); onetry needs multi-peer P2P mode.",
	"addwitnessaddress":            "Add a P2SH-wrapped witness address for a known script (DogeGo: witness path not available - error).",
	"abandontransaction":           "Remove an unconfirmed wallet transaction from the mempool and persist as abandoned (built-in wallet).",
	"backupwallet":                 "Copies wallet.json to a destination path (built-in wallet when enabled).",
	"bumpfee":                      "Increase fee of a BIP125 replaceable transaction (built-in wallet auto-bump from change, or options.rawtx).",
	"psbtbumpfee":                  "Build and sign a fee-bumped replacement as PSBT (built-in wallet; does not broadcast).",
	"clearbanned":                  "Clears all banned IPs (in-memory when BanManager is wired).",
	"combinerawtransaction":        "Combine raw hex transactions into one; inputs must match except scriptSig (DogeGo: legacy only, no witness).",
	"createauxblock":               "Create merge-mined block template (hash, chainid, target, coinbasevalue) for a P2PKH payout address.",
	"createmultisig":               "Creates a P2SH multisig address from a list of hex-encoded public keys and required signatures.",
	"createpsbt":                   "Creates an unsigned PSBT (base64) from inputs/outputs; fills prevouts when txindex/raw blocks/mempool available.",
	"createrawtransaction":         "Creates a serialized, hex-encoded raw transaction from inputs and outputs.",
	"converttopsbt":                "Converts a hex-encoded legacy transaction to PSBT (base64); rejects scriptSig unless permitsigdata=true.",
	"decoderawtransaction":         "Returns an object representing the serialized, hex-encoded transaction.",
	"decodepsbt":                   "Returns a JSON object representing a serialized PSBT (hex or base64; legacy txs only).",
	"analyzepsbt":                  "Analyzes a PSBT: per-input has_utxo/is_final/missing, fee when UTXOs are filled (legacy subset).",
	"combinepsbt":                  "Combines multiple PSBTs with the same unsigned transaction (base64 strings).",
	"joinpsbts":                    "Joins distinct PSBTs (different inputs/outputs) into one PSBT; duplicate prevouts are rejected.",
	"finalizepsbt":                 "Applies final_scriptSig fields; returns hex when complete and extract=true (default).",
	"utxoupdatepsbt":               "Adds non_witness_utxo for inputs from txindex/raw blocks/mempool when available.",
	"decodescript":                 "Decodes a hex-encoded script (redeemScript or P2SH redeem).",
	"disconnectnode":               "Disconnects from the specified peer (DogeGo: requires DataPaths.DisconnectNode).",
	"dogego_importbip38":           "Decrypts a BIP38 encrypted private key and imports the address (passphrase; optional rescan). Supports non-EC, EC-multiply, and lot/sequence keys.",
	"dogego_importmnemonic":        "Restores the HD wallet from a BIP39 mnemonic (optional passphrase; optional rescan; replaces existing HD seed).",
	"dogego_importwalletdat":       "Migrates keys from Core wallet.dat via native BDB read (unencrypted, or encrypted with options.passphrase), Core dumpwallet (core_rpc_addr + via_core_rpc), or a dumpwallet text file. Native import may return pool_count, pool_entries (spends_key_matched per row), pool_keys_matched/unmatched, pool_unmatched_entries, pool_unmatched_hint, pool_index_min/max, pool_indices_replayed (true when matched HD receive pubkeys are reserved into hd_keypool), pool_core_indices_stored (count of hd_keypool_core_index entries written), keypool_hint, keypool_refill_size when pool-only rows remain, and runs keypoolrefill on HD wallets when Core keypool entries are present.",
	"dogego_listwalletaddresses":   "Lists tracked wallet addresses from wallet.json (label, HD path, watch-only, imported flags, iskeypool, isnodetip, hd_keypool_core_index when stored).",
	"dogego_probewalletdat":        "Inspects a Core wallet.dat (native BDB): encrypted flag, key/watch/pool counts, pool_pubkeys, pool_entries (index+pubkey, spends_key_matched), pool_keys_matched, pool_keys_unmatched, pool_unmatched_entries, pool_unmatched_hint, pool_index_min/max, pool_indices_replayed (false on probe; true after import when HD replay succeeds), encrypted_keys, needs_passphrase, can_import hint - no import. When a built-in HD wallet is wired, also returns hd_keypool_core_index and pool_core_indices_stored from wallet.json.",
	"dogego_recoverheaders":        "Rewinds stale header journal data (compressed-period fix + one retarget window) without deleting headers.bin.",
	"dogego_verifypqcommitment":    "Off-chain verify Phase-1 OP_RETURN PQ commitment script (FLC1/DIL2/RCG4); params: script_hex or tag + commitment_hex.",
	"dogego_listextensions":        "Lists built-in and installed DogeGo extensions (manifest, enabled state).",
	"dogego_listextensioncatalog":  "Lists remote GitHub catalog merged with local install state (optional force refresh param).",
	"dogego_enableextension":       "Enables an extension by id (sandbox host; no wallet access).",
	"dogego_disableextension":      "Disables an extension by id.",
	"dogego_instextensionzip":      "Installs an extension from a zip containing dogego.extension.json.",
	"dogego_instextensionurl":      "Installs an extension from an https zip URL (optional sha256 param).",
	"dogego_instextension":         "Installs an extension from the remote catalog by id.",
	"dogego_uninstextension":       "Uninstalls a non-builtin extension (optional remove_data bool).",
	"dogego_ext_dogego_zkl2_info":              "ZK L2 extension status: P2P peers, groth16 VK summary, proof index samples.",
	"dogego_ext_dogego_zkl2_installdefaultvk":  "Install bundled demo Groth16 verifying key to data/vk/default.vk.",
	"dogego_ext_dogego_zkl2_submitproof":   "Submit tx-anchored ZK proof after validation (relays zkinv to zkproof-v1 peers).",
	"dogego_ext_dogego_zkl2_verifyproof":   "Verifies a tx-anchored proof off-L1 (optional verifying_key or verifying_key_chunks for inline VK).",
	"dogego_ext_dogego_zkl2_checkzkp":      "Alias for verifyproof (OP_CHECKZKP analogue off L1).",
	"dogego_createpqcarrier":       "Build unsigned TX_C + TX_R PQ carrier plan from funded tx_base_hex (requires setwalletflag pq_carrier).",
	"dogego_sendpqcarrier":         "Fund, sign, and broadcast TX_C + TX_R PQ carrier payment (requires setwalletflag pq_carrier).",
	"dogego_verifypqcarrier":       "Verifier-side TX_C + TX_R carrier validation (commitment, TX_R linkage, optional PQ sig via pqcrypto); params: object with txc_hex, txr_hex, pk_script_hex. Returns valid, pq_verify (passed/failed/skipped), commitment_match, linkage_ok.",
	"dumptxoutset":                 "Writes a JSON-lines UTXO snapshot from the in-memory UTXO cache at tip.",
	"scantxoutset":                 "Scans the in-memory UTXO set for addr/pkh/sh/raw/multisig descriptors (start/abort/status; synchronous).",
	"scanblocks":                   "Scans block heights using BIP158 basic filters for matching descriptors (start/abort/status; optional filter_false_positives verify).",
	"loadtxoutset":                 "Loads a dumptxoutset JSON-lines file into the in-memory UTXO cache at chainActive tip.",
	"dumpprivkey":                  "Reveals the WIF private key for the built-in testnet wallet address.",
	"dumpwallet":                   "Dumps wallet WIF and watch scripts to a text file (built-in wallet when enabled).",
	"echo":                         "Returns the given arguments unchanged (testing / compatibility).",
	"echojson":                     "Returns the given arguments with JSON types preserved.",
	"encryptwallet":                "Encrypts wallet.json spend keys with a passphrase (scrypt + AES-GCM).",
	"enumeratesigners":             "Lists HWI-compatible external signers when signer_cmd is configured (empty array when unset).",
	"estimatefee":                  "Estimates approximate fee in DOGE/kB to confirm within nblocks (DogeGo heuristic).",
	"estimatepriority":             "Deprecated in Core; returns -1 (coin-age priority estimator removed in Core).",
	"estimatesmartfee":             "Smart fee estimate with conservative/economical modes (DogeGo subset).",
	"estimatesmartpriority":        "Deprecated in Core; returns priority -1 with blocks and note.",
	"fundrawtransaction":           "Fund a raw tx from wallet/UTXO cache; options: changeAddress, fee_rate, conf_target, lockUnspents, includeWatching (solvable imported P2SH multisig is included by default), add_inputs, minimumTotalFee, replaceable.",
	"getdescriptorinfo":            "Analyze an output descriptor (addr / pkh / sh(pkh) / sh(multi) / sh(cltv|csv multi|pkh) / multi; issolvable when wallet holds enough keys).",
	"deriveaddresses":              "Derive addresses from a supported output descriptor (non-range subset; bare multi has no address).",
	"extractdescriptor":            "Split a descriptor into canonical form and checksum (supported descriptor subset).",
	"importdescriptors":            "Import addr / pkh / sh(pkh) / sh(multi) / sh(cltv|csv multi|pkh) / multi descriptors; optional keys[] for spendable import.",
	"estimaterawfee":               "Estimate fee rate for a raw tx (smart fee + optional unsigned hex size scaling).",
	"generate":                     "Mine up to nblocks blocks immediately (delegates to generatetoaddress when MiningAddress is configured).",
	"generatetoaddress":            "Mine legacy blocks to a P2PKH address (scrypt PoW); merge-mined heights need createauxblock.",
	"getaccount":                   "Returns the account name for an address (DogeGo: deprecated; no address book - empty string if address is valid P2PKH).",
	"getaccountaddress":            "Returns the current receive address for the default account (HD: next keypool address without issuing a new one).",
	"getaddressinfo":               "Returns detailed information about a Dogecoin address (built-in wallet: mine/watch flags, desc when known, pubkey when known, iskeypool for unused HD receive/change keypool entries, isnodetip for the dedicated node-tip HD key, hd_keypool_core_index when Core pool index was stored on import); P2SH multisig redeem loaded from wallet when imported.",
	"getaddednodeinfo":             "Returns information about manually added peers (DogeGo: from DataPaths.AddedNodes).",
	"getaddrmaninfo":               "Returns addrman-style summary of learned peer addresses (DogeGo: tried/new counts and bucket stats).",
	"getnodeaddresses":             "Returns known peer addresses from the addrbook (Core getnodeaddresses; count 0 = all).",
	"getaddressesbyaccount":        "Returns addresses for an account label (DogeGo: deprecated; no wallet - empty array; rejects account \"*\").",
	"getaddressesbylabel":          "Returns a Core-shaped object of addresses that have the given label; each entry includes purpose receive or send (change).",
	"getauxblock":                  "Submit merge-mined auxpow (hash, auxpow hex); use createauxblock to create templates (no wallet).",
	"getbestblockhash":             "Returns the hash of the best (tip) block in the most-work fully-validated chain.",
	"getblock":                     "Returns block data by hash or height; verbosity controls JSON vs hex.",
	"getblockchaininfo":            "Returns various state information regarding blockchain processing.",
	"getblockcount":                "Returns the number of blocks in the longest blockchain.",
	"getblockfilter":               "Returns BIP158 basic block filter for a block (requires tx index + raw blocks).",
	"getblockfilterheader":         "Returns BIP158 basic block filter header for a block (requires tx index + raw blocks).",
	"getblockhash":                 "Returns hash of block in best-block-chain at index provided.",
	"getblockheader":               "Returns serialized block header or JSON header object.",
	"getblockstats":                "Compute per-block statistics from stored raw blocks; fee fields when prevouts resolve via UTXO cache or tx index.",
	"getblocktemplate":             "Returns BIP22 block template with chain context and mempool transaction selection; proposal mode validates without storing.",
	"getbalance":                   "Returns confirmed balance for the built-in wallet (optional include_watchonly for watch-only UTXOs).",
	"getbalances":                  "Returns mine and watchonly balance buckets (trusted, untrusted_pending, immature).",
	"getchaintips":                 "Returns information about all known tips in the block tree (DogeGo: header journal view).",
	"getchaintxstats":              "Returns statistics about the chain's transaction volume (window_tx_count from rawblocks when stored; txcount from tx index).",
	"getconnectioncount":           "Returns the number of P2P connections (relay/primary plus block-assist and dedicated header-sync links during IBD).",
	"getdeploymentinfo":            "Returns consensus deployment state at tip or optional block hash (buried + BIP9; period statistics when started).",
	"getdifficulty":                "Returns the current difficulty as a multiple of the minimum difficulty.",
	"getindexinfo":                 "Returns index status: native indexes/tx, block filters, and coinstatsindex hash when UTXO cache is at chainActive tip.",
	"reindexblockfilters":          "Rebuilds all persisted BIP158 basic block filters from raw blocks (requires tx index).",
	"reindextx":                    "Rebuilds indexes/tx from all rawblocks (v2 embedded tx). Optional clear (boolean) to wipe index first.",
	"upgradetxindex":               "Upgrades legacy 36-byte indexes/tx files to v2 (embedded tx raw). Optional max_files (default 10000). Full rebuild: reindextx.",
	"getinfo":                      "Deprecated aggregate of network / chain / wallet-style fields (DogeGo: non-wallet subset).",
	"getmempoolancestors":          "Returns all in-mempool ancestors for a transaction id.",
	"getmempooldescendants":        "Returns all in-mempool descendants for a transaction id.",
	"getmempoolentry":              "Returns mempool data for a transaction id.",
	"getmempoolinfo":               "Returns details on the node's active state regarding the transaction memory pool (incl. feerate_percentiles when prevouts resolve).",
	"setmempoolpaused":             "Pauses or resumes acceptance of new transactions into the mempool (existing txs remain).",
	"mempoolexists":                "Returns true if a transaction id is in the mempool.",
	"savemempool":                  "Writes the in-memory mempool to dogego_mempool.json under the chain data directory.",
	"saveutxosnapshot":             "Writes the in-memory UTXO set to utxo.cache in the background (chainActive checkpoint before restart; poll dogego_utxo_snapshot_height).",
	"syncutxo":                     "Starts bounded chainActive connect replay in the background (optional max blocks 1-64, default 8). Poll blocks on getblockchaininfo.",
	"syncutxocache":                "Alias for syncutxo (bounded UTXO cache replay from stored bodies).",
	"loadmempool":                  "Reloads transactions from dogego_mempool.json through current mempool policy (returns loaded/skipped counts).",
	"importmempool":                "Imports transactions from a DogeGo mempool JSON dump file (not Core mempool.dat).",
	"getmemoryinfo":                "Returns an object containing information about memory usage.",
	"getmininginfo":                "Returns a json object containing mining-related information.",
	"getmocktime":                  "Get the current mocktime (0 when using the real clock).",
	"getnetworkinfo":               "Returns an object containing various state info regarding P2P networking.",
	"getnetworkhashps":             "Returns the estimated network hashes per second based on recent blocks.",
	"getnewaddress":                "Returns a new BIP44 receive address (m/44'/3'/0'/0/n); optional label and legacy address_type.",
	"getrawchangeaddress":          "Returns a new change address (consumes HD internal keypool).",
	"listdescriptors":              "List active wallet descriptors (pkh / sh(pkh) / sh(multi) / multi for built-in HD wallet).",
	"setwalletflag":                "Set wallet flags (avoid_reuse, pq_commitments, pq_carrier) on the built-in wallet.",
	"getnettotals":                 "Returns information about network traffic (bytes sent/received).",
	"getpeerinfo":                  "Returns data about each connected network node (DogeGo: subset when peers exist).",
	"getrawmempool":                "Returns mempool txids (array) or verbose=true object keyed by txid (Core-shaped entries).",
	"getrawtransaction":            "Returns raw transaction hex or JSON (native indexes/tx, then mempool); optional verbose and blockhash filter.",
	"getreceivedbyaccount":         "Returns total received for an account (built-in wallet: default \"\" account from UTXO cache).",
	"getreceivedbyaddress":         "Returns total amount received by an address (optional include_watchonly; built-in wallet UTXO cache).",
	"getreceivedbylabel":           "Returns total amount received for a label (built-in wallet: UTXO cache).",
	"gettransaction":               "Get detailed information about an in-wallet transaction (built-in wallet: hex from mempool/chain; fee on mempool sends).",
	"gettxout":                     "Returns details about an unspent transaction output.",
	"gettxspendingprevout":         "Scans the mempool for transactions spending the given outputs (Core-shaped spendingtxid field).",
	"gettxoutproof":                "Returns a hex-encoded proof that a txid was included in a block (DogeGo subset).",
	"gettxoutsetinfo":              "Returns statistics about the unspent transaction output set (DogeGo: live counts from in-memory UTXO cache when synced to tip).",
	"getzmqnotifications":          "Returns active ZMQ PUB notification endpoints (Core-compatible type/address/hwm).",
	"getunconfirmedbalance":        "Returns the wallet's total unconfirmed balance (DogeGo: no wallet - always 0).",
	"getwalletinfo":                "Returns wallet state for the built-in testnet wallet (balance from UTXO cache). HD wallets may include hd_keypool_core_index and pool_core_indices_stored after Core wallet.dat import. DogeGo extensions: wallet_index_height, chain_active_height, needs_rescan, rescan_from_height, dogego_wallet_scan_index_ok, dogego_wallet_history_fast_path, dogego_wallet_listtransactions_utxo_walk, dogego_wallet_listtransactions_scan_pending, dogego_wallet_history_deferred, dogego_wallet_history_defer_reason when scan metadata is wired; scanning while rescan is in flight; signer_cmd_configured when external signer transport is set.",
	"verifytxoutproof":             "Verifies a hex-encoded proof and returns the transaction ids it commits to.",
	"getrpcinfo":                   "Returns RPC server state: authentication_failures, method map, active_commands (empty in DogeGo).",
	"help":                         "List all commands, or get help for a specified command.",
	"invalidateblock":              "Disconnects the chain before the block and marks it invalid (persisted in chain_policy.json); notifies waitfor* at new chainActive tip.",
	"importaddress":                "Imports watch-only P2PKH/P2SH address, hex script, or P2SH redeem script; optional label; optional rescan (SyncUtxo + block scan).",
	"importmulti":                  "Batch-import watch-only addresses/scripts or pkh/sh(pkh)/sh(multi)/sh(cltv|csv multi)/multi desc (built-in wallet; optional label; options.rescan runs SyncUtxo + block scan).",
	"importprivkey":                "Imports a WIF (optional label, rescan); replaces spend key when same address; else cosigner in extra_privkeys_hex.",
	"importprunedfunds":            "Imports a pruned transaction and merkle proof (verifies CMerkleBlock; credits built-in wallet watch outputs).",
	"importpubkey":                 "Imports a hex-encoded public key as watch-only P2PKH; optional rescan (SyncUtxo + block scan).",
	"importwallet":                 "Imports keys from a dumpwallet-style text file (WIF, watch scripts, labels); SyncUtxo + block scan after import.",
	"keypoolrefill":                "Refills HD receive and change keypools up to newsize (built-in wallet; Core TopUpKeyPool fill-to-target; no-op when already at target).",
	"listaccounts":                 "Deprecated object of account balances (built-in wallet: default \"\" account from UTXO cache).",
	"listaddressgroupings":         "Lists address groups inferred from the wallet (DogeGo: no wallet - empty array).",
	"listbanned":                   "List all banned IPs/subnets (BanManager when wired).",
	"listlabels":                   "Returns unique address labels from the built-in wallet (wallet.json).",
	"listlockunspent":              "Returns locked outpoints (built-in wallet; persisted in wallet.json).",
	"listreceivedbyaccount":        "List incoming payments by account (built-in wallet: default \"\" account; txids from UTXO cache).",
	"listreceivedbyaddress":        "List incoming payments grouped by address (built-in wallet: UTXO cache; txids per address).",
	"listreceivedbylabel":          "List incoming payments grouped by address label (built-in wallet; txids per label).",
	"liststucktransactions":        "Lists unconfirmed wallet sends not in the mempool (built-in wallet; UTXO-cache light path; verbose adds hex/fee).",
	"listsinceblock":               "Lists wallet transactions since a block hash (include_watchonly, target_confirmations; lastblock at chainActive).",
	"listtransactions":             "Returns recent wallet transactions (built-in wallet: UTXO-cache light path + mempool; bip125-replaceable when unconfirmed).",
	"listunspent":                  "Returns unspent outputs for the built-in wallet (UTXO cache; include_unsafe; query_options minimumAmount/maximumCount).",
	"lockunspent":                  "Lock or unlock outputs for coin selection (built-in wallet; fundrawtransaction / listunspent respect locks).",
	"move":                         "Deprecated: move balance between account labels (built-in wallet: no-op bookkeeping success).",
	"ping":                         "Requests that a ping be sent to all other nodes; see getpeerinfo pingtime / minping / pingwait.",
	"preciousblock":                "Marks a block preferred when competing headers have equal chain work.",
	"prioritisetransaction":        "Adjusts virtual fee delta for a mempool tx (affects template selection and eviction ordering).",
	"pruneblockchain":              "Deletes raw block files below height (headers.bin kept; indexes/tx entries for pruned blocks removed).",
	"reconsiderblock":              "Removes a block from the invalid set so it may be accepted again on sync.",
	"removeprunedfunds":            "Deletes an abandoned or importprunedfunds transaction from the built-in wallet (wallet.json).",
	"rescan":                       "SyncUtxo + block scan for wallet scripts from optional start height (persists scanned_txs in wallet.json).",
	"resendwallettransactions":     "Re-broadcast unconfirmed transactions (built-in wallet: wallet sends from UTXO-cache light rows; else all mempool txs).",
	"sendfrom":                     "Send from account label to address (built-in wallet; optional 7th param fund options JSON).",
	"sendmany":                     "Send to multiple addresses in one tx (built-in wallet; optional 6th param fund options JSON).",
	"sendrawtransaction":           "Submits raw transaction to mempool and peers; optional allowhighfees and maxfeerate (DOGE/kB).",
	"submitpackage":                "Submits a topologically sorted package of raw txs (child last); CPFP package feerate; returns package_msg, tx-results (fees.effective-includes), and replaced-transactions when BIP125 replaces.",
	"sendtoaddress":                "Send to an address (built-in wallet; optional 6th param fund options: fee_rate, conf_target, replaceable, pqcommit when pq_commitments wallet flag is on, …).",
	"setaccount":                   "Deprecated: set account label for an address (built-in wallet: no-op for tracked addresses).",
	"setlabel":                     "Assign a label to a tracked wallet address (persisted in wallet.json).",
	"setban":                       "Attempts to add or remove an IP/subnet from the ban list.",
	"setmaxconnections":            "Sets the maximum outbound P2P connections including the primary peer (8-32 when multi-peer mode is active).",
	"setmocktime":                  "Set the local unix clock used for header validation (0 clears mock time).",
	"setnetworkactive":             "Disable/enable all P2P network activity (when SetNetworkActive is wired).",
	"settxfee":                     "Sets the transaction fee per kB in wallet.json (built-in wallet; used by fundrawtransaction).",
	"signmessage":                  "Sign a message with the built-in wallet address (Core compact signature format).",
	"signmessagewithprivkey":       "Sign a message with the private key of an address (compact signature, Core message format).",
	"signrawtransaction":           "Sign legacy P2PKH / P2PK / P2SH multisig inputs (P2SH prevtx needs redeemScript; wallet can auto-fill prevtxs).",
	"signrawtransactionwithkey":    "Sign legacy tx inputs with explicit WIF keys (no wallet key merge).",
	"simulaterawtransaction":       "Estimates wallet balance change (DOGE) from broadcasting raw hex transactions.",
	"signrawtransactionwithwallet": "Sign with built-in wallet keys (P2PKH, P2SH multisig when cosigner keys + watch_redeems are present); auto prevouts from UTXO cache.",
	"signerdisplayaddress":         "Shows a descriptor receive address on the configured external signer (signer_cmd).",
	"submitauxblock":               "Submit solved auxpow for a createauxblock template (returns true when accepted).",
	"submitblock":                  "Stores a block when its header is in the journal, or extends the journal by one header when the block builds on the tip; runs ConnectBlock when tx index is enabled; relays P2P block to the outbound peer when connected.",
	"stop":                         "Stop the server (graceful shutdown when DataPaths.Shutdown is wired).",
	"testmempoolaccept":            "Returns result of mempool acceptance tests without broadcasting; optional maxfeerate in DOGE/kB.",
	"truncatetoheight":             "Truncates headers, raw blocks, tx index, and UTXO cache to height (destructive operator maintenance).",
	"uptime":                       "Returns the total uptime of the server in seconds.",
	"validateaddress":              "Returns information about the given dogecoin address; optional redeemScript hex for P2SH multisig validation. Built-in wallet: iskeypool for unused HD receive/change keypool entries, isnodetip for the dedicated node-tip HD key, and hd_keypool_core_index when a Core wallet.dat pool index was stored on import.",
	"verifychain":                  "Verifies headers (level 3+: Digishield/auxpow with headers_aux.bin); level 4 ConnectBlock over native rawblocks. Optional third param verbose=true returns RPC error text on failure.",
	"verifymessage":                "Verify a signed message (Core compact signature over message hash).",
	"walletcreatefundedpsbt":       "Creates and funds a PSBT from wallet UTXOs; fills prevouts and BIP32 deriv paths for HD wallet inputs/outputs.",
	"walletlock":                   "Locks an encrypted built-in wallet.",
	"descriptorprocesspsbt":        "Alias for walletprocesspsbt (Core descriptor wallet PSBT signing).",
	"walletprocesspsbt":            "Updates a PSBT with wallet prevouts, BIP32 deriv paths for HD keys, and signs (final_scriptSig; optional hex when complete).",
	"walletpassphrase":             "Unlocks encrypted wallet.json for timeout seconds.",
	"walletpassphrasechange":       "Changes passphrase on an encrypted built-in wallet.",
	"waitforblock":                 "Waits for a block hash to become the chain tip (hidden in Core).",
	"waitforblockheight":           "Waits for the chain to reach at least the given height (hidden in Core).",
	"waitfornewblock":              "Waits for a new tip block after the call (hidden in Core).",
}

// MethodHelp returns the one-line help string for a registered RPC command.
func MethodHelp(method string) (string, bool) {
	h, ok := rpcMethodHelp[strings.TrimSpace(method)]
	return h, ok
}

func execHelp(params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 1 {
		return nil, -32602, "Too many arguments"
	}
	if len(params) == 0 {
		return strings.Join(SupportedMethods(), "\n"), 0, ""
	}
	var cmd string
	if err := json.Unmarshal(params[0], &cmd); err != nil {
		return nil, -32602, "command must be a string"
	}
	cmd = strings.TrimSpace(cmd)
	if h, ok := rpcMethodHelp[cmd]; ok {
		return h, 0, ""
	}
	return "help: unknown command: " + cmd, 0, ""
}

func execHelpWithExtensions(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 1 {
		return nil, -32602, "Too many arguments"
	}
	if len(params) == 0 {
		return strings.Join(SupportedMethods(), "\n"), 0, ""
	}
	var cmd string
	if err := json.Unmarshal(params[0], &cmd); err != nil {
		return nil, -32602, "command must be a string"
	}
	cmd = strings.TrimSpace(cmd)
	if h, ok := rpcMethodHelp[cmd]; ok {
		return h, 0, ""
	}
	if h, ok := ExtensionRPCHelp(paths, cmd); ok {
		return h, 0, ""
	}
	return "help: unknown command: " + cmd, 0, ""
}
