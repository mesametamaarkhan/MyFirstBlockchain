// main.go
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Block defines the block structure
type Block struct {
	Index        int      `json:"index"`
	Timestamp    int64    `json:"timestamp"`
	Transactions []string `json:"transactions"`
	MerkleRoot   string   `json:"merkle_root"`
	PrevHash     string   `json:"prev_hash"`
	Hash         string   `json:"hash"`
	Nonce        int64    `json:"nonce"`
	Difficulty   int      `json:"difficulty"`
}

// Blockchain and pending txs (in-memory)
var (
	blockchain          []Block
	pendingTransactions []string
	mutex               = &sync.Mutex{}
	defaultDifficulty   = 4 // number of leading zeros required
)

// --- Helper: SHA256 hex
func sha256hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// --- Merkle tree functions
// computeMerkleRoot accepts slice of tx strings and returns hex merkle root.
// Simple approach: hash leaves, pairwise combine and hash up to root.
// If odd number, duplicate last.
func computeMerkleRoot(txs []string) string {
	if len(txs) == 0 {
		return sha256hex("") // empty root
	}
	// leaf hashes
	var layer []string
	for _, t := range txs {
		layer = append(layer, sha256hex(t))
	}
	for len(layer) > 1 {
		var next []string
		for i := 0; i < len(layer); i += 2 {
			if i+1 == len(layer) {
				// duplicate last
				combined := layer[i] + layer[i]
				next = append(next, sha256hex(combined))
			} else {
				combined := layer[i] + layer[i+1]
				next = append(next, sha256hex(combined))
			}
		}
		layer = next
	}
	return layer[0]
}

// computeHash of a block (without Hash field)
func computeHash(b Block) string {
	record := strconv.Itoa(b.Index) +
		strconv.FormatInt(b.Timestamp, 10) +
		b.PrevHash +
		b.MerkleRoot +
		strconv.FormatInt(b.Nonce, 10) +
		strconv.Itoa(b.Difficulty)
	return sha256hex(record)
}

// --- Proof of Work (simple): find nonce s.t. hash has difficulty leading zeros
func mineBlock(b Block, stopAfterMs int64) (Block, error) {
	prefix := strings.Repeat("0", b.Difficulty)
	start := time.Now()
	var nonce int64
	for {
		b.Nonce = nonce
		hash := computeHash(b)
		if strings.HasPrefix(hash, prefix) {
			b.Hash = hash
			return b, nil
		}
		nonce++
		// optional safety to avoid runaway loops: allow stopAfterMs ms
		if stopAfterMs > 0 && time.Since(start) > time.Duration(stopAfterMs)*time.Millisecond {
			return b, fmt.Errorf("mining timed out after %d ms (last nonce %d)", stopAfterMs, nonce)
		}
	}
}

// --- Genesis block creation
func createGenesisBlock() Block {
	gen := Block{
		Index:        0,
		Timestamp:    time.Now().Unix(),
		Transactions: []string{"Genesis Block"},
		PrevHash:     "",
		Difficulty:   defaultDifficulty,
	}
	gen.MerkleRoot = computeMerkleRoot(gen.Transactions)
	// Mine genesis (so Hash and Nonce set)
	mined, err := mineBlock(gen, 0)
	if err != nil {
		// fallback: set hash manually
		gen.Nonce = 0
		gen.Hash = computeHash(gen)
		return gen
	}
	return mined
}

// --- Blockchain functions
func getLastBlock() Block {
	return blockchain[len(blockchain)-1]
}

func addBlock(transactions []string, difficulty int) (Block, error) {
	mutex.Lock()
	defer mutex.Unlock()
	prev := getLastBlock()
	newBlock := Block{
		Index:        prev.Index + 1,
		Timestamp:    time.Now().Unix(),
		Transactions: transactions,
		PrevHash:     prev.Hash,
		Difficulty:   difficulty,
	}
	newBlock.MerkleRoot = computeMerkleRoot(newBlock.Transactions)
	mined, err := mineBlock(newBlock, 0)
	if err != nil {
		return Block{}, err
	}
	blockchain = append(blockchain, mined)
	return mined, nil
}

// --- HTTP Handlers

// Simple CORS middleware
func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*") // for demo; tighten in production
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// Add Transaction: POST /tx  { "data": "some string" }
func handleAddTx(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	type req struct {
		Data string `json:"data"`
	}
	var body req
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Data) == "" {
		http.Error(w, "invalid body, expected {\"data\":\"...\"}", http.StatusBadRequest)
		return
	}
	mutex.Lock()
	pendingTransactions = append(pendingTransactions, body.Data)
	mutex.Unlock()
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":              "transaction added",
		"pending_transactions": pendingTransactions,
	})
}

// Mine block: POST /mine  optional JSON { "difficulty": 4, "timeout_ms": 0 }
func handleMine(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	type req struct {
		Difficulty int   `json:"difficulty"`
		TimeoutMs  int64 `json:"timeout_ms"` // optional safe timeout in ms
	}
	var body req
	// default values
	body.Difficulty = defaultDifficulty
	body.TimeoutMs = 0
	_ = json.NewDecoder(r.Body).Decode(&body) // ignore error, we have defaults

	mutex.Lock()
	if len(pendingTransactions) == 0 {
		mutex.Unlock()
		http.Error(w, "no pending transactions to mine", http.StatusBadRequest)
		return
	}
	txs := make([]string, len(pendingTransactions))
	copy(txs, pendingTransactions)
	// clear pending txs before mining to avoid duplicates (in real world you'd lock & validate)
	pendingTransactions = []string{}
	mutex.Unlock()

	block, err := addBlock(txs, body.Difficulty)
	if err != nil {
		http.Error(w, "mining failed: "+err.Error(), http.StatusInternalServerError)
		// If mining failed, return txs back to pending
		mutex.Lock()
		pendingTransactions = append(pendingTransactions, txs...)
		mutex.Unlock()
		return
	}
	json.NewEncoder(w).Encode(block)
}

// Get blockchain: GET /blocks
func handleGetBlocks(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	mutex.Lock()
	defer mutex.Unlock()
	json.NewEncoder(w).Encode(blockchain)
}

// Get pending transactions: GET /pending
func handleGetPending(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	mutex.Lock()
	defer mutex.Unlock()
	json.NewEncoder(w).Encode(pendingTransactions)
}

// Search: GET /search?q=...  returns array of matches {blockIndex, tx}
func handleSearch(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	q := r.URL.Query().Get("q")
	if strings.TrimSpace(q) == "" {
		http.Error(w, "query param q required", http.StatusBadRequest)
		return
	}
	type match struct {
		BlockIndex  int    `json:"block_index"`
		Transaction string `json:"transaction"`
		Hash        string `json:"block_hash"`
	}
	var results []match
	mutex.Lock()
	for _, b := range blockchain {
		for _, tx := range b.Transactions {
			if strings.Contains(strings.ToLower(tx), strings.ToLower(q)) {
				results = append(results, match{
					BlockIndex:  b.Index,
					Transaction: tx,
					Hash:        b.Hash,
				})
			}
		}
	}
	mutex.Unlock()
	json.NewEncoder(w).Encode(results)
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	fmt.Fprintf(w, "Simple Go Blockchain API\nAvailable endpoints:\nPOST /tx {data}\nPOST /mine {difficulty?}\nGET /blocks\nGET /pending\nGET /search?q=...\n")
}

// --- main
func main() {
	// initialize chain with genesis
	gen := createGenesisBlock()
	blockchain = append(blockchain, gen)
	fmt.Println("Genesis block created:", gen.Hash)

	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/tx", handleAddTx)
	http.HandleFunc("/mine", handleMine)
	http.HandleFunc("/blocks", handleGetBlocks)
	http.HandleFunc("/pending", handleGetPending)
	http.HandleFunc("/search", handleSearch)

	addr := ":8080"
	fmt.Printf("Listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
