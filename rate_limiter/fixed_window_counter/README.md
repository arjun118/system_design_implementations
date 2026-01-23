# working

- divide the timeline into fix-sized time windows and assign a counter for each window
- each req increments the counter by one
- once the counter reaches the pre-defined threshold, new requests are dropped until a new time window starts
