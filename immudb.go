package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/codenotary/immudb/pkg/api/schema"
	immuclient "github.com/codenotary/immudb/pkg/client"
	"log"
	"os"
	"strconv"
	"time"
)

type immuConnection struct {
	hostname string
	port     int
	username string
	password string
	database string
	token    string
	ctx      context.Context
	client   immuclient.ImmuClient
}

func get_env_default(varname, default_value string) string {
	ret, ok := os.LookupEnv(varname)
	if !ok {
		ret = default_value
	}
	return ret
}

func (ic *immuConnection) cfg_init() {
	var err error
	ic.hostname = get_env_default("IF_IMMUDB_HOSTNAME", "127.0.0.1")
	ic.port, err = strconv.Atoi(get_env_default("IF_IMMUDB_PORT", "3322"))
	if err != nil {
		log.Fatalln("Unable to parse port number")
	}
	ic.username = get_env_default("IF_IMMUDB_USERNAME", "immudb")
	ic.password = get_env_default("IF_IMMUDB_PASSWORD", "immudb")
	ic.database = get_env_default("IF_IMMUDB_DATABASE", "defaultdb")
	ic.client = nil
}

func (ic *immuConnection) context() context.Context {
	return ic.ctx
}

func (ic *immuConnection) connect(ctx context.Context) {
	log.Printf("Connecting to immudb: %s:%d", ic.hostname, ic.port)
	ic.ctx = ctx
	opts := immuclient.DefaultOptions().WithAddress(ic.hostname).WithPort(ic.port)
	ic.client = immuclient.NewClient().WithOptions(opts)
	err := ic.client.OpenSession(ic.ctx, []byte(ic.username), []byte(ic.password), ic.database)
	if err != nil {
		log.Fatalln("Failed to open session. Reason:", err.Error())
	}
	log.Printf("Connected")
}

func (ic *immuConnection) pushmsg(msgs []logMsg) error {
	l := len(msgs)
	ops := make([]*schema.Op, 3*l)
	c := 0
	for i, msg := range msgs {
		key := []byte(fmt.Sprintf("L:%s/%s@%s:%f:%d",
			msg.Kubernetes.Namespace,
			msg.Kubernetes.PodName,
			msg.Kubernetes.Host,
			msg.Date,
			i,
		))
		ptr := []byte(fmt.Sprintf("T:%f:%d", msg.Date, i))
		val, _ := json.Marshal(msg)
		c, ops[c] = c+1, &schema.Op{
			Operation: &schema.Op_Kv{
				Kv: &schema.KeyValue{
					Key:   key,
					Value: val,
				},
			},
		}
		c, ops[c] = c+1, &schema.Op{
			Operation: &schema.Op_Ref{
				Ref: &schema.ReferenceRequest{
					Key:           ptr,
					ReferencedKey: key,
				},
			},
		}
		if msg.AssignedId != "" {
			idPtr := []byte(fmt.Sprintf("I:%s", msg.AssignedId))
			c, ops[c] = c+1, &schema.Op{
				Operation: &schema.Op_Ref{
					Ref: &schema.ReferenceRequest{
						Key:           idPtr,
						ReferencedKey: key,
					},
				},
			}
		}
	}
	
	opList := &schema.ExecAllRequest{Operations: ops[:c]}
	id, err := ic.DoExecAll(opList)
	if err != nil {
		log.Printf("Error sending data to immudb: %s", err.Error())
		return err
	}
	log.Printf("Sent data, tx %d", id)
	return nil
}

func (ic *immuConnection) DoExecAll(opList *schema.ExecAllRequest) (uint64, error) {
	var err error
	for i:=0; i<5; i++ {
		ret, err := ic.client.ExecAll(ic.ctx, opList)
		if err == nil {
			return ret.Id, nil
		}
		ic.client.CloseSession(ic.ctx)
		time.Sleep(time.Duration(i * 100) * time.Millisecond)
		ic.connect(ic.ctx)
	}
	// one last time...
	ret, err := ic.client.ExecAll(ic.ctx, opList)
	if err == nil {
		return ret.Id, nil
	}
	return 0, err
}
