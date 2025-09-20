// src/App.jsx
import React, { useEffect, useState } from "react";

const API = "http://localhost:8080";

function App() {
  const [pending, setPending] = useState([]);
  const [blocks, setBlocks] = useState([]);
  const [txInput, setTxInput] = useState("");
  const [message, setMessage] = useState("");
  const [difficulty, setDifficulty] = useState(4);
  const [searchQ, setSearchQ] = useState("");
  const [searchResults, setSearchResults] = useState([]);

  async function fetchPending() {
    const res = await fetch(`${API}/pending`);
    const data = await res.json();
    setPending(data);
  }
  async function fetchBlocks() {
    const res = await fetch(`${API}/blocks`);
    const data = await res.json();
    setBlocks(data);
  }

  useEffect(() => {
    fetchPending();
    fetchBlocks();
    const iv = setInterval(() => {
      fetchPending();
      fetchBlocks();
    }, 3000);
    return () => clearInterval(iv);
  }, []);

  async function addTx(e) {
    e.preventDefault();
    if (!txInput.trim()) return;
    const res = await fetch(`${API}/tx`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ data: txInput }),
    });
    if (res.ok) {
      setMessage("Transaction added");
      setTxInput("");
      fetchPending();
    } else {
      const txt = await res.text();
      setMessage("Error: " + txt);
    }
  }

  async function mine() {
    setMessage("Mining... this may take a while depending on difficulty");
    const res = await fetch(`${API}/mine`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ difficulty: Number(difficulty) }),
    });
    if (res.ok) {
      const blk = await res.json();
      setMessage(`Mined block #${blk.index} hash ${blk.hash}`);
      fetchBlocks();
      fetchPending();
    } else {
      const txt = await res.text();
      setMessage("Mine failed: " + txt);
      fetchPending();
      fetchBlocks();
    }
  }

  async function doSearch(e) {
    e.preventDefault();
    if (!searchQ.trim()) return;
    const res = await fetch(`${API}/search?q=${encodeURIComponent(searchQ)}`);
    if (res.ok) {
      const data = await res.json();
      setSearchResults(data);
    } else {
      setSearchResults([]);
      setMessage("Search failed: " + (await res.text()));
    }
  }

  return (
    <div style={{ maxWidth: 1000, margin: "20px auto", fontFamily: "Inter, sans-serif" }}>
      <h1>Mesam Blockchain</h1>

      <section style={{ border: "1px solid #ddd", padding: 12, borderRadius: 8, marginBottom: 12 }}>
        <h2>Add Transaction</h2>
        <form onSubmit={addTx}>
          <input
            value={txInput}
            onChange={(e) => setTxInput(e.target.value)}
            placeholder="Transaction string"
            style={{ width: "70%", padding: 8 }}
          />
          <button style={{ padding: "8px 12px", marginLeft: 8 }}>Add Tx</button>
        </form>
        <p style={{ color: "green" }}>{message}</p>
        <div style={{ marginTop: 8 }}>
          <strong>Pending Transactions:</strong>
          <ul>
            {(pending || []).map((p, i) => (
              <li key={i}>{p}</li>
            ))}
          </ul>
        </div>
      </section>

      <section style={{ border: "1px solid #ddd", padding: 12, borderRadius: 8, marginBottom: 12 }}>
        <h2>Mine Block</h2>
        <label>
          Difficulty (hex leading zeros):{" "}
          <input
            type="number"
            value={difficulty}
            onChange={(e) => setDifficulty(e.target.value)}
            style={{ width: 60 }}
            min={1}
            max={8}
          />
        </label>
        <div style={{ marginTop: 8 }}>
          <button onClick={mine} style={{ padding: "8px 12px" }}>
            Mine
          </button>
        </div>
      </section>

      <section style={{ border: "1px solid #ddd", padding: 12, borderRadius: 8, marginBottom: 12 }}>
        <h2>Search Blockchain</h2>
        <form onSubmit={doSearch}>
          <input
            value={searchQ}
            onChange={(e) => setSearchQ(e.target.value)}
            placeholder="search text"
            style={{ width: "60%", padding: 8 }}
          />
          <button style={{ padding: "8px 12px", marginLeft: 8 }}>Search</button>
        </form>
        <div>
          <h4>Results:</h4>
          <ul>
            {Array.isArray(searchResults) && searchResults.length > 0 ? (
              searchResults.map((r, idx) => (
                <li key={idx}>
                  Block #{r.block_index} — {r.transaction}{" "}
                  (hash: {r.hash ? r.hash.slice(0, 12) : "N/A"}...)
                </li>
              ))
            ) : (
              <li>No results found</li>
            )}
          </ul>
        </div>
      </section>

      <section style={{ border: "1px solid #ddd", padding: 12, borderRadius: 8 }}>
        <h2>Blockchain (latest first)</h2>
        <button onClick={fetchBlocks} style={{ marginBottom: 8 }}>
          Refresh
        </button>
        {blocks.length === 0 ? (
          <p>No blocks yet</p>
        ) : (
          blocks
            .slice()
            .reverse()
            .map((b) => (
              <div key={b.index} style={{ borderTop: "1px solid #eee", paddingTop: 12, marginTop: 12 }}>
                <div>
                  <strong>Index:</strong> {b.index} &nbsp; <strong>Timestamp:</strong>{" "}
                  {new Date(b.timestamp * 1000).toLocaleString()}
                </div>
                <div>
                  <strong>Hash:</strong> {b.hash}
                </div>
                <div>
                  <strong>PrevHash:</strong> {b.prev_hash}
                </div>
                <div>
                  <strong>Merkle Root:</strong> {b.merkle_root}
                </div>
                <div>
                  <strong>Nonce:</strong> {b.nonce} &nbsp; <strong>Difficulty:</strong> {b.difficulty}
                </div>
                <div>
                  <strong>Transactions:</strong>
                  <ul>
                    {b.transactions.map((t, i) => (
                      <li key={i}>{t}</li>
                    ))}
                  </ul>
                </div>
              </div>
            ))
        )}
      </section>
    </div>
  );
}

export default App;
