package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.etcd.io/etcd/clientv3"
)

func main() {
	var values []byte
	// 800B
	for i := 0; i < 100; i++ {
		values = append(values, []byte("mockyaml")...)
	}
	value := string(values)

	generateValue := func() string {
		return fmt.Sprintf("%s: %s", time.Now(), value)
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"http://127.0.0.1:12380"},
	})
	if err != nil {
		log.Fatal(err)
	}

	_, err = client.MemberList((context.Background()))
	if err != nil {
		log.Fatal(err)
	}

	wg := sync.WaitGroup{}
	wg.Add(10)
	for node := 0; node < 10; node++ {
		go func(node int) {
			defer wg.Done()
			for revision := 0; revision < 100000; revision++ {
				var ops []clientv3.Op
				for key := 0; key < 200; key++ {
					key := fmt.Sprintf("/status/config-%d/node-%d", key, node)
					ops = append(ops, clientv3.OpPut(key, generateValue()))

				}
				_, err = client.Txn(context.Background()).Then(ops...).Commit()
				if err != nil {
					log.Fatal(err)
				}
				if revision%1000 == 0 {
					fmt.Printf("node:%d revision:%d\n", node, revision)
				}
			}
		}(node)
	}

	wg.Wait()
}
