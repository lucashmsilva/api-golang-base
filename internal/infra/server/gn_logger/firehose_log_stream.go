package gn_logger

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/firehose"
	"github.com/aws/aws-sdk-go-v2/service/firehose/types"
)

const (
	MAX_LOG_BYTE_LENGTH     = 1000 * 1024     // 1000 KB
	MAX_RECORDS_BYTE_LENGTH = 4 * 1024 * 1024 // 4 MB

	// customizable via options
	MAX_RECORD_BATCH_SIZE    = 500
	DEFAULT_WATCHER_MS_DELAY = 1000
)

type FirehoseWriteStreamOptions struct {
	StreamName   string
	MaxBatchSize *int
	WatcherDelay *int
}

type FirehoseWriteStream struct {
	options        *FirehoseWriteStreamOptions
	recordsBuff    []types.Record
	firehoseClient *firehose.Client
	ticker         *time.Ticker
	mu             sync.Mutex
}

func New(opts *FirehoseWriteStreamOptions) (*FirehoseWriteStream, error) {
	var watcherDelay int
	var cfg aws.Config
	var ticker *time.Ticker

	if opts == nil || opts.WatcherDelay == nil {
		watcherDelay = DEFAULT_WATCHER_MS_DELAY
	} else {
		watcherDelay = *opts.WatcherDelay
	}

	if opts == nil || opts.MaxBatchSize == nil {
		defaultMaxBatchSize := MAX_RECORD_BATCH_SIZE
		opts.MaxBatchSize = &defaultMaxBatchSize
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	ticker = time.NewTicker(time.Millisecond * time.Duration(watcherDelay))

	firehoseStream := &FirehoseWriteStream{
		options:        opts,
		recordsBuff:    []types.Record{},
		firehoseClient: firehose.NewFromConfig(cfg),
		ticker:         ticker,
		mu:             sync.Mutex{},
	}

	go firehoseStream.startWatcher(ticker)

	return firehoseStream, nil
}

func (f *FirehoseWriteStream) startWatcher(ticker *time.Ticker) {
	for range ticker.C {
		f.send()
	}
}

func (f *FirehoseWriteStream) send() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.recordsBuff) == 0 {
		return 0
	}

	var records []types.Record
	var recordsByteLength int

	if len(f.recordsBuff) >= *f.options.MaxBatchSize {
		records = f.recordsBuff[:*f.options.MaxBatchSize]
	} else {
		records = f.recordsBuff[:]
	}

	f.recordsBuff = f.recordsBuff[len(records):]

	for _, v := range records {
		recordsByteLength += len(v.Data)
	}

	// If the list of records to be sent has a byte length bigger than the limit,
	// pop the last record and add it to the front of the buffer. Repeat until the list fits.
	for recordsByteLength > MAX_RECORDS_BYTE_LENGTH {
		if len(records) == 0 {
			return 0
		}

		lastRecord := records[len(records)-1]
		records = records[:len(records)-1]
		records = slices.Insert(records, 0, lastRecord)

		recordsByteLength -= len(lastRecord.Data)
	}

	input := &firehose.PutRecordBatchInput{
		DeliveryStreamName: &f.options.StreamName,
		Records:            records,
	}

	response, err := f.firehoseClient.PutRecordBatch(context.TODO(), input)
	// In case of errors from AWS, add the entire record list back to the buffer
	if err != nil {
		f.recordsBuff = append(f.recordsBuff, records...)
		fmt.Printf("Error sending logs to firehose: %v]\n", err)
		return 0
	}

	if *response.FailedPutCount == int32(0) {
		return len(records)
	}

	// If any record failed to be sent, add them back to the buffer
	failedRecords := make([]types.Record, 0, *response.FailedPutCount)
	for i, r := range response.RequestResponses {
		if r.ErrorCode != nil {
			failedRecords = append(failedRecords, records[i])
		}
	}

	f.recordsBuff = append(f.recordsBuff, failedRecords...)

	return len(records) - int(*response.FailedPutCount)
}

func (f *FirehoseWriteStream) Close() error {
	f.ticker.Stop()

	for len(f.recordsBuff) > 0 {
		f.send()
	}

	return nil
}

func (f *FirehoseWriteStream) Write(logBytes []byte) (n int, err error) {
	if len(logBytes) > MAX_LOG_BYTE_LENGTH {
		fmt.Printf("Log length exceeds %v B. log: %v", MAX_LOG_BYTE_LENGTH, string(logBytes))
		return len(logBytes), nil
	}

	f.recordsBuff = append(f.recordsBuff, types.Record{Data: logBytes})

	if len(f.recordsBuff) >= *f.options.MaxBatchSize {
		go f.send()
	}

	return len(logBytes), nil
}
