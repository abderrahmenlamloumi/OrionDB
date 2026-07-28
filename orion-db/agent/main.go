package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	pb "orion-db/schema"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

var stats struct {
	success, exhausted, deadline, unavail, other uint64
}

func main() {
	addr := flag.String("addr", "127.0.0.1:9090", "OrionDB address")
	workers := flag.Int("workers", 100, "Concurrent workers")
	duration := flag.Duration("duration", 20*time.Second, "Benchmark duration")
	flag.Parse()

	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	defer conn.Close()

	client := pb.NewCollectorClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	start := time.Now()
	log.Printf("Starting load simulation (%d workers, %v)...", *workers, *duration)

	// Live statistics
	go func() {
		for t := time.NewTicker(time.Second); ; <-t.C {
			if ctx.Err() != nil {
				return
			}
			s := atomic.LoadUint64(&stats.success)
			e := atomic.LoadUint64(&stats.exhausted) + atomic.LoadUint64(&stats.deadline) + atomic.LoadUint64(&stats.unavail) + atomic.LoadUint64(&stats.other)
			fmt.Printf("[Stats] Elapsed: %.1fs | Success: %d | Errors: %d | Rate: %.0f req/sec\n", time.Since(start).Seconds(), s, e, float64(s)/time.Since(start).Seconds())
		}
	}()

	// Worker pool
	var wg sync.WaitGroup
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

			for ctx.Err() == nil {
				reqCtx, reqCancel := context.WithTimeout(context.Background(), time.Second)
				_, err := client.SubmitTelemetry(reqCtx, &pb.TelemetryPoint{
					Metric:    "system_memory_cache_bytes",
					Value:     rng.Float64() * 1e9,
					Timestamp: time.Now().UnixNano(),
					Labels: map[string]string{
						"service": []string{"checkout-api", "auth-service", "payment-processor", "user-gateway"}[rng.Intn(4)],
						"region":  []string{"eu-west-3", "us-east-1", "ap-southeast-1"}[rng.Intn(3)],
						"env":     []string{"production", "staging", "development"}[rng.Intn(3)],
						"pod":     fmt.Sprintf("pod-%d-%x", rng.Intn(50000), rng.Int31()),
					},
				})
				reqCancel()

				if err == nil {
					atomic.AddUint64(&stats.success, 1)
					continue
				}

				if st, ok := status.FromError(err); ok {
					switch st.Code() {
					case codes.ResourceExhausted:
						atomic.AddUint64(&stats.exhausted, 1)
					case codes.DeadlineExceeded:
						atomic.AddUint64(&stats.deadline, 1)
					case codes.Unavailable:
						atomic.AddUint64(&stats.unavail, 1)
					default:
						if atomic.AddUint64(&stats.other, 1) <= 5 {
							log.Printf("[DEBUG] gRPC Error: code=%s msg=%s", st.Code(), st.Message())
						}
					}
				} else if atomic.AddUint64(&stats.other, 1) <= 5 {
					log.Printf("[DEBUG] Non-gRPC Error: %v", err)
				}
			}
		}(i)
	}
	wg.Wait()

	el := time.Since(start).Seconds()
	s, ex, d, u, o := stats.success, stats.exhausted, stats.deadline, stats.unavail, stats.other
	errs := ex + d + u + o
	tot := float64(s + errs)

	fmt.Printf("\n==============================\nSimulation Finished\n==============================\n"+
		"Success:              %d\nResourceExhausted:    %d\nDeadlineExceeded:     %d\nUnavailable:          %d\n"+
		"Other Errors:         %d\nTotal Errors:         %d\nAverage Throughput:   %.2f req/sec\n"+
		"Success Rate:         %.2f%%\nError Rate:           %.2f%%\n==============================\n",
		s, ex, d, u, o, errs, float64(s)/el, 100*float64(s)/tot, 100*float64(errs)/tot)
}
