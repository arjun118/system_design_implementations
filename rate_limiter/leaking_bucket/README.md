# notes

## algorithm

- requests are processed at a fixed rate
- FIFO queue
- when a request arrives,we check if the queue is full
  - if yes: drop the request
  - if no: add the request to the queue
  - requests are pulled from the queue and processed at regular intervals

## notes on testing
- refer `TestLeakAllowRequests`, `TestLeakAllowRequestsGPTVersion`
- Concurrency tests mutexes
- Time tests rate limiters
- still need robust tests
