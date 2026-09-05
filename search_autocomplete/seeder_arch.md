                 ORIGINAL FILE
                 ~10M rows
                      │
                      ▼
             Normalize leading junk
                      │
                      ▼
             External lexical sort
                      │
                      ▼
          Streaming duplicate aggregation
                      │
                      ▼
              canonical_sorted.txt
                      │
                      ▼
        ┌─────────────────────────────┐
        │ recursive memory-aware      │
        │ prefix index construction   │
        └─────────────────────────────┘
                      │
             ┌────────┴────────┐
             ▼                 ▼
         MongoDB             local
          batches             temp

1. next steps 2. split into first rune buckets 3. then split again basis memory considerations

lets start from the root

- for every first char - we need to get range
