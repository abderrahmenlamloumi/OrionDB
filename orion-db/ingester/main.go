package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	pb "orion-db/schema"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"orion-db/internal/buffer"
	"orion-db/internal/index"

	//"orion-db/internal/index"
	"orion-db/internal/ingest"
	"orion-db/internal/storage"
)

type server struct {
	pb.UnimplementedCollectorServer
	s      *storage.LSM
	series *index.SeriesRegistry
	tags   *index.TagIndex
	ring   *buffer.RingBuffer
}

func metricFromTelemetry(req *pb.TelemetryPoint) *ingest.Metric {
	metric := ingest.AcquireMetric()
	metric.Name = req.GetMetric()
	metric.Value = req.GetValue()
	metric.Timestamp = req.GetTimestamp()

	if metric.Timestamp == 0 {
		metric.Timestamp = time.Now().UnixNano()
	}

	if len(req.GetLabels()) > 0 {
		metric.Tags = make([]ingest.Tag, 0, len(req.GetLabels()))
		for k, v := range req.GetLabels() {
			metric.Tags = append(metric.Tags, ingest.Tag{Key: k, Value: v})
		}
	}
	return metric
}

func (s *server) SubmitTelemetry(ctx context.Context, req *pb.TelemetryPoint) (*pb.TelemetryPoint, error) {
	metric := metricFromTelemetry(req)

	if !s.ring.Enqueue(metric) {
		// Now the client will correctly register this as ResourceExhausted!
		return nil, status.Error(codes.ResourceExhausted, "ingestion queue full")
	}
	return req, nil
}

func (s *server) runConsumer(wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		metric := s.ring.Dequeue()
		if metric == nil {
			time.Sleep(5 * time.Millisecond)
			continue
		}

		seriesID := s.series.GetOrCreate(metric.Name, labelsFromMetric(metric))

		for _, tag := range metric.Tags {
			s.tags.Add(fmt.Sprintf("%s=%s", tag.Key, tag.Value), seriesID)
		}

		if err := s.s.Put(seriesID, metric.Timestamp, metric.Value); err != nil {
			log.Printf("lsm write error: %v", err)
		}

		tags := make([]string, 0, len(metric.Tags))
		for _, tag := range metric.Tags {
			tags = append(tags, fmt.Sprintf("%s=%s", tag.Key, tag.Value))
		}
		//log.Printf("ingested metric=%s series=%d value=%.2f timestamp=%d tags=%v", metric.Name, seriesID, metric.Value, metric.Timestamp, tags)
		ingest.ReleaseMetric(metric)
	}
}

func labelsFromMetric(metric *ingest.Metric) map[string]string {
	labels := make(map[string]string, len(metric.Tags))
	for _, tag := range metric.Tags {
		labels[tag.Key] = tag.Value
	}
	return labels
}

func consumerCount() int {
	if value := os.Getenv("ORION_INGEST_CONSUMERS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			return parsed
		}
	}

	return 4
}

func main() {
	listener, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}

	storageDir := os.Getenv("ORION_STORAGE_DIR")
	if storageDir == "" {
		storageDir = "data"
	}

	lsm, err := storage.OpenLSM(storage.Config{
		Dir:                 storageDir,
		MemtableMaxEntries:  1024,
		CompactionThreshold: 4,
	})
	if err != nil {
		log.Fatalf("failed to open LSM: %v", err)
	}
	defer lsm.Close()

	srv := grpc.NewServer()
	s := &server{
		s:      lsm,
		series: index.NewSeriesRegistry(),
		tags:   index.NewTagIndex(),
		ring:   buffer.NewRingBuffer(4096),
	}
	var wg sync.WaitGroup
	consumers := consumerCount()
	wg.Add(consumers)
	for i := 0; i < consumers; i++ {
		go s.runConsumer(&wg)
	}

	pb.RegisterCollectorServer(srv, s)
	fmt.Println("ingester listening on :9090")
	if err := srv.Serve(listener); err != nil {
		log.Fatalf("serve failed: %v", err)
	}

	wg.Wait()
}
