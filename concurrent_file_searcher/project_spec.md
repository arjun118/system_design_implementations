## 📂 Project Requirements: `gosearch`

### 1. Functional Requirements
* **CLI Arguments**: The program should accept two flags or arguments:
    1.  `path`: The root directory to start the search.
    2.  `query`: The string to look for in filenames.
* **Recursive Search**: It must look through all subdirectories, no matter how deep they are.
* **Performance**: It must use goroutines to process directories in parallel.
* **Output**: Print the full path of every matching file to the console as soon as it is found.

### 2. Technical Challenges (The "Concurrency" Part)
* **The WaitGroup**: You need to track how many "searchers" are currently running. If you exit too early, you miss files; if you don't exit at all, you have a deadlock.
* **Channel Communication**: Use a channel to stream found file paths from the background goroutines back to the `main` function for printing.
* **Throttling (Optional/Pro)**: If you start a goroutine for *every* single file in a massive directory (like `/`), you might crash the OS by opening too many file descriptors. Think about how you might limit the number of active workers.

### 3. Suggested Architecture
1.  **Main Goroutine**: Responsible for starting the initial search and listening on a `results` channel.
2.  **Worker Logic**: A function that reads a directory. For every file, it checks the name. For every subdirectory, it launches *another* instance of itself in a new goroutine.
3.  **The "Done" Signal**: You'll need to coordinate when to close the `results` channel so the `main` function knows the search is officially over.



---

## 🛠 Your Go Toolkit
To build this, you should aim to use:
* `os.ReadDir` to list files in a folder.
* `sync.WaitGroup` to manage the lifecycle of your goroutines.
* `chan string` to pass the results back.
* `path/filepath` to handle joining directory paths correctly across different OSs.
