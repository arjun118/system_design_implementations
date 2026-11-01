One distinction: if the status update itself fails—for example, the handler succeeded but Ack failed—don’t treat that as an ordinary handler retry. Log and surface it as an infrastructure/reporting error; the server may eventually redeliver the job, so handlers should be designed for possible duplicate execution.

