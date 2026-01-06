# Mutex usage and correctness
1. lock only to access shared data structured (map here)
2. never lock while doing request handling or blocking operations
