# inside AddServer
- current: we should not sort the entire slice after every server addition
- actual way to implement: insert the virtualnode hash at the correct position [avoided, reasoning below]
```
This is a brilliant algorithmic question. You are thinking exactly like a systems engineer, trying to shave off those $O(N \log N)$ cycles.However, your analysis missed one massive, hidden cost of how slices work under the hood in Go (and arrays in almost all languages like C++).The short answer is: Your current approach (Append + Sort) is actually drastically faster than Binary Search + Insert. Here is the breakdown of why your proposed approach is a trap, and what the actual Big-O math looks like.The Hidden Trap: Slice Insertion is $O(N)$In Go, a slice is backed by a contiguous block of memory (an array).When you want to insert a single hash into the middle of a sorted slice, you can certainly find the index in $O(\log N)$ using binary search.But to actually put the item there, Go has to take every single element to the right of that index and physically copy/shift it one spot over in memory to make room. This memory shift (memmove under the hood) takes $O(N)$ time.Let's do the MathLet $N$ be the total number of nodes already on the ring (e.g., 3,000).Let $V$ be the number of new virtual nodes you are adding (e.g., 300).Scenario A: Your Proposed Idea (Binary Search + Insert)For each of your 300 virtual nodes, you:Binary search: $O(\log N)$Insert and shift memory: $O(N)$Total time for one node: $O(N + \log N) \approx O(N)$Total time for $V$ nodes: $O(V \cdot N)$If you have 10 servers (3,000 nodes) and add an 11th, you are performing 300 binary searches and forcing the CPU to shift blocks of 3,000 integers in memory 300 separate times. That is incredibly expensive.Scenario B: Your Current Code (Append + Sort)Append 300 items to the end of the slice: $O(V)$ amortized.Sort the entire slice of size $N+V$: $O((N+V) \log(N+V))$Total time: $O((N+V) \log(N+V))$Why Scenario B (Your Current Code) WinsAt first glance, $O(V \cdot N)$ vs $O(N \log N)$ might look competitive, but Scenario B has a secret weapon: Go's sorting algorithm.Since Go 1.19, slices.Sort uses an algorithm called pdqsort (Pattern-Defeating Quicksort). Pdqsort is notoriously brilliant at recognizing arrays that are already mostly sorted.Because your existing HashRing is perfectly sorted, and you only appended 300 random items to the end, pdqsort doesn't do a full $O(N \log N)$ scramble. It recognizes the sorted prefix and essentially does a highly optimized, bulk-insertion routine under the hood. It is blazingly fast and shifts memory in bulk, rather than one-by-one.The "True" Optimal Approach: The $O(N)$ MergeIf you want to absolutely max out your DSA skills and beat Go's internal slices.Sort, there is one way to do it. It borrows from the "Merge" step of Merge Sort.Generate your 300 new hashes.Sort only those 300 new hashes: $O(V \log V)$.Allocate a brand new slice of size $N+V$.Use two pointers to iterate through the old HashRing and your newly sorted hashes, picking the smaller of the two and adding it to the new slice.Total time: $O(V \log V + N)$.This is mathematically strictly faster than your current code, and does zero overlapping memory shifts!The Verdict:Unless you are running a cluster with millions of virtual nodes, the two-pointer merge is probably overkill. Your current append + slices.Sort is highly idiomatic, takes advantage of Go's pdqsort optimizations, and is perfectly suited for production
```
- 
# wrap the app in a cli
- good distribution visualization [done]
- learn to use bubble tea and lipgloss [later]
# remove server
- current complexity: O(V * N)
- to be approach: rebuild approach (need to study about this)
# remove duplicate rehashing (by storing more)


# choice of hash function decides distribution evenness

# use mutexes to make this concurrency safe RWMutex and write tests

# never hold a mutex lock while doing network i/o or disk i/o
