# Immufluent

Sends logs generated by fluentbit to immudb.

Supports rotating database, so you can easily prune old logs.

## Usage
```
Usage of ./immufluent:
      --address string           Binding address (default "0.0.0.0")
      --autorotate int           Interval for internal rotation (seconds) (default 86400)
      --buffer-delay int         max buffer delay (milliseconds) (default 100)
      --buffer-size int          max buffer size (default 100)
      --immudb-hostname string   immudb server address (default "127.0.0.1")
      --immudb-password string   immudb admin password (default "immudb")
      --immudb-pattern string    database pattern name (with strftime variables) (default "log_%Y_%m")
      --immudb-port int          immudb server port (default 3322)
      --immudb-username string   immudb admin username (default "immudb")
      --port int                 Listening port (default 8090)
```

Configure immufluent using command line arguments or Environment variables. Every command line option has a matching env variable prefixed with `IF_`, uppercased and with `-` converted to `_`.

So you can set immudb address using `IF_IMMUDB_HOSTNAME`.

## Log collection
Configure fluentbit with the `http` output, sending logs to immufluent IP address and poirt, setting the path to the `/log` endpoint. Format must be `json`:
```ini
[OUTPUT]
    Name http
    Match kube.*
    Host <ip_address_of_immufluent>
    Port 8090
    Uri /log
    Format json
```

## Rotation

`--immudb-pattern` is used to generate the database name, using strftime expansion (see https://pkg.go.dev/github.com/lestrrat-go/strftime). TO actually switch the logs to the new database,
you have to *rotate* the database. When a rotation is invoked, the name is generated again and, if different than the current one, old one will be closed and new one will be used.
That is attempted automatically every `--autorotate` seconds, or can be done maually calling endpoint `/rotate`.
Note that setting `autorotate` to `0` will disable autorotation.