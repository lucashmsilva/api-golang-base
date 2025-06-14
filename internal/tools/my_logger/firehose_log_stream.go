package my_logger

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

// Default buffer params
const (
	// Firehose hard limits
	max_log_byte_length     = 1000 * 1024     // 1000 KB
	max_records_byte_length = 4 * 1024 * 1024 // 4 MB

	// Customizable via options
	max_record_batch_size    = 500
	default_watcher_ms_delay = 1000
)

type FirehoseLogStreamOptions struct {
	// Firehose stream name as configured in AWS
	StreamName string

	// Record buffer size
	MaxBatchSize *int

	// Time between automatic record buffer flushes
	WatcherDelay *int

	// Instead of sending records trough the AWS API, print them to stdout
	Debug bool
}

type FirehoseLogStream struct {
	options        FirehoseLogStreamOptions
	recordsBuff    []types.Record
	firehoseClient firehoseClient
	ticker         *time.Ticker
	mu             sync.Mutex
}

// Interface to allow mocking of the AWS Firehose API
type firehoseClient interface {
	PutRecordBatch(ctx context.Context, input *firehose.PutRecordBatchInput, optFns ...func(*firehose.Options)) (*firehose.PutRecordBatchOutput, error)
}

type firehoseDebugClient struct {
	_ aws.Config
}

func (f *firehoseDebugClient) PutRecordBatch(ctx context.Context, input *firehose.PutRecordBatchInput, _ ...func(*firehose.Options)) (*firehose.PutRecordBatchOutput, error) {
	for _, v := range input.Records {
		fmt.Print(string(v.Data))
	}

	return &firehose.PutRecordBatchOutput{FailedPutCount: aws.Int32(0)}, nil
}

func NewFirehoseLogStream(opts FirehoseLogStreamOptions) (*FirehoseLogStream, error) {
	var watcherDelay int
	var cfg aws.Config
	var firehoseClient firehoseClient

	if opts.WatcherDelay == nil {
		watcherDelay = default_watcher_ms_delay
	} else {
		watcherDelay = *opts.WatcherDelay
	}

	if opts.MaxBatchSize == nil {
		defaultMaxBatchSize := max_record_batch_size
		opts.MaxBatchSize = &defaultMaxBatchSize
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	firehoseClient = firehose.NewFromConfig(cfg)
	if opts.Debug {
		firehoseClient = &firehoseDebugClient{cfg}
	}

	firehoseStream := &FirehoseLogStream{
		options:        opts,
		recordsBuff:    []types.Record{},
		firehoseClient: firehoseClient,
		ticker:         time.NewTicker(time.Millisecond * time.Duration(watcherDelay)),
	}

	go func() {
		for range firehoseStream.ticker.C {
			go firehoseStream.send()
		}
	}()

	return firehoseStream, nil
}

func (f *FirehoseLogStream) Write(logBytes []byte) (n int, err error) {
	if len(logBytes) > max_log_byte_length {
		fmt.Printf("log length exceeds %v B.\n", max_log_byte_length)
		return len(logBytes), nil
	}

	go func(r types.Record) {
		f.mu.Lock()
		defer f.mu.Unlock()

		f.recordsBuff = append(f.recordsBuff, r)
		if len(f.recordsBuff) >= *f.options.MaxBatchSize {
			go f.send()
		}

	}(types.Record{Data: slices.Clone(logBytes)})

	return len(logBytes), nil
}

func (f *FirehoseLogStream) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.ticker.Stop()

	for len(f.recordsBuff) > 0 {
		f.send()
	}

	return nil
}

func (f *FirehoseLogStream) send() int {
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
	for recordsByteLength > max_records_byte_length {
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

	response, err := f.firehoseClient.PutRecordBatch(context.TODO(), input) // putRecordBatchMock(context.TODO(), input)
	if err != nil {
		// In case of errors from AWS, add the entire record list back to the buffer
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
