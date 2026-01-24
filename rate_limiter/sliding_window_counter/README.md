# working

- algo keep strack of request timestamps. usually kept in cache - eg: sorted sets of redis
- new request come in
  - remove all the outdated timestamps
  - outdated: old than the start of the current time window
  - add timestamp of the new request to the log
  - log_size is same or lower than the allowed count, request is accepted otherwise rejected
