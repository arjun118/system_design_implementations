
For a Mini-Redis or a modern KV store, you have two main architectural choices: **B+ Trees** (Read-optimized, like Postgres/MySQL) or **LSM Trees** (Write-optimized, like Cassandra/LevelDB). Since you're using Go, an **LSM Tree (Log-Structured Merge-Tree)** is much more idiomatic and fun to build.

---

## 🏗️ Project Spec: `golevel-engine`

### 1. Architectural Components
An LSM engine consists of three primary layers that move data from memory to disk.

#### A. The Write-Ahead Log (WAL)
* **Purpose:** Recovery.
* **Logic:** Every `SET` command is first appended to a raw binary file on disk before anything else happens.
* **Requirement:** If the process crashes, the engine must read the WAL on startup to rebuild the in-memory state.

#### B. The MemTable
* **Purpose:** High-speed in-memory storage.
* **Data Structure:** Use a **Skip List** or a **Balanced Binary Tree (AVL/Red-Black)**.
* **Logic:** Data stays here until the MemTable reaches a size limit (e.g., 4MB). Once full, it becomes "Immutable" and a background goroutine flushes it to disk.

#### C. SSTables (Sorted String Tables)
* **Purpose:** Persistent, ordered storage on disk.
* **Format:** A binary file containing key-value pairs sorted by key.
* **Components:**
    1.  **Data Block:** The actual KV pairs.
    2.  **Index Block:** Offsets telling you where each key starts (so you don't have to scan the whole file).
    3.  **Bloom Filter:** A probabilistic data structure used to quickly tell if a key *might* exist in this file without reading the disk.



---

### 2. Functional Requirements
* **`Put(key, value)`**: Writes to WAL, then updates MemTable.
* **`Get(key)`**: 
    1.  Check active MemTable.
    2.  Check Immutable MemTables (waiting to be flushed).
    3.  Check Bloom Filters of SSTables.
    4.  If Bloom Filter hits, search SSTable Index, then Read Data Block.
* **`Delete(key)`**: In LSM trees, you don't delete data immediately. You write a **Tombstone** (a special marker saying the key is deleted). The actual data is removed later during compaction.

---

### 3. The "Complex" Concurrency Challenges
To make this a true Go project, you must handle these background orchestrations:

* **The Flush Pipeline:** When a MemTable is full, you shouldn't block the user. You should swap in a fresh, empty MemTable and send the full one to a **Flusher Goroutine** via a channel.
* **Compaction (The Background Cleaner):** Over time, you'll have dozens of SSTable files. Some contain old versions of the same key. A **Compaction Goroutine** must periodically merge smaller SSTables into larger ones, discarding old values and tombstones.
* **Read/Write Locking:** The MemTable needs a `sync.RWMutex`, but the SSTables are immutable—once written, they never change, which makes concurrent reads very fast and easy.



---

### 4. Technical Constraints (The "Pro" Touches)
* **Key/Value Encoding:** Don't store strings. Store `[]byte`. Use **Varints** (Variable-length integers) to store the lengths of keys and values to save disk space.
* **Sparse Indexing:** You don't need to index every key. Index every 100th key. To find a key in between, find the closest index and scan forward. This saves massive amounts of RAM.

---
