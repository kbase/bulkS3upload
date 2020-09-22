package main

//
// Tool for bulk upload of files form local filesystem into an S3 compatible
// service. Takes an input file with a list of files to upload, including the
// path in the S3 service. All files go into the same bucket, and the files
// in the S3 bucket match the path fragment listed in the file.
// The config parameter rootDir is a prefix that is added to the path of
// the files in the filelist. This rootDir file prefix is not passed to the bucket,
// the files are copied into the root of the bucket.
//
// sychan@lbl.gov 8/2019
//
import (
	"bufio"
	"fmt"
	"context"
	"github.com/minio/minio-go"
	"github.com/minio/minio-go/pkg/credentials"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

)

type CopyResult struct {
	path  string
	bytes int64
	err   error
}

// Configuration defaults for location of file and default settings
const configFile = ".bulkS3upload"
const configFileType = "yaml"

var configPath = []string{"$HOME", "."}

var confDefaults = map[string]string{
	"rootDir":       "./",
	"maxWorkers":    "1",
	"timerInterval": "3.0",
}

// Configuration settings - globally scoped, not a big deal in this situation
var rootDir string
var maxWorkers int
var endpoints []string
var accessKeyID string
var secretAccessKey string
var bucket string
var timerInterval float64
var debug bool
var ssl bool

var elapsed time.Duration
var lineCount = 0
var lastLineCount = 0
var totalBytes int64
var lastTotalBytes int64
var startTime = time.Now()
var errorLines = 0

// Worker routine that initializes a minio client with an endpoint and a destination bucket
// and then waits on a channel for file paths that should be copied into the endpoint/bucket
func copyWorker(bucket string, url string, accessID string, secretKey string, ssl bool, files <-chan string, nodeStats chan<- CopyResult, wg *sync.WaitGroup) {

	ctx := context.Background()
	defer wg.Done()

	minioClient, err := minio.New(url, &minio.Options{
		Creds: credentials.NewStaticV4(accessID, secretKey, ""),
		Secure: ssl,
	})
	if err != nil {
		log.Fatalln(err)
	}
	count := 0
	for filePath := range files {
		stringArray := strings.Split(filePath,"/")
		objectPath := stringArray[0] + "/" + stringArray[1] + "/" + stringArray[2] + "/" + stringArray[3]
		fullPath := rootDir + filePath
		uploadInfo, err := minioClient.FPutObject(ctx, bucket, objectPath, fullPath, minio.PutObjectOptions{})
		if err != nil {
			log.Printf(err.Error())
		}
		log.Printf("ETag: %s VersionID: %s", uploadInfo.ETag,uploadInfo.VersionID)
		nodeStats <- CopyResult{path: filePath, bytes: 0, err: err}
		count++
	}
}

// Worker routine that is given a file to read, and a channel to write each line to.
// Once the input file is finished, close the channel
func fileList(srcFilePath string, files chan<- string) {
	file, err := os.Open(srcFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		shockNode := scanner.Text()
		files <- shockNode
		count++
	}
	close(files)
	log.Printf("Read in %d lines", count)
}

func printStats() {
	elapsed = time.Since(startTime)
	bytesPerSec := int64(float64(totalBytes) / elapsed.Seconds())
	lastBytesPerSec := int(float64(totalBytes-lastTotalBytes) / float64(timerInterval))
	lastTotalBytes = totalBytes
	filesPerSec := int(float64(lineCount) / elapsed.Seconds())
	lastFilesPerSec := int(float64(lineCount-lastLineCount) / timerInterval)
	lastLineCount = lineCount
	fmt.Printf("%6.0fs, %d files ( %d err), %d bytes, %d bytes/s, %d files/s, lastinterval: %d bytes/s %d files/s\n",
		elapsed.Seconds(), lineCount, errorLines, totalBytes, bytesPerSec, filesPerSec, lastBytesPerSec, lastFilesPerSec)
}

func intervalStats(ticker <-chan time.Time) {
	for _ = range ticker {
		printStats()
	}
}

func accumulateResults(nodeStats <-chan CopyResult, done chan<- bool) {
	for node := range nodeStats {
		if debug {
			fmt.Printf("Read stats for %s size %d\n", node.path, node.bytes)
		}
		lineCount++
		totalBytes += node.bytes
		if node.err != nil {
			errorLines++
		}
	}
	done <- true
}

func readConfig() {
	viper.SetConfigName(configFile)
	viper.SetConfigType(configFileType)
	for _, cPath := range configPath {
		viper.AddConfigPath(cPath)
	}
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error
			fmt.Printf("Warning: %s\n", err)
		} else {
			log.Fatalln(err)
		}
	}
	pflag.String("rootDir", confDefaults["rootDir"], "Base directory on local filesystem for objects to tbe moved")
	m, _ := strconv.Atoi(confDefaults["maxWorkers"])
	pflag.Int("maxWorkers", m, "Number of workers to start (must be less than # of files to copy")
	pflag.StringSlice("endpoints", strings.Split(confDefaults["endpoints"], ","), "List of ip:port S3 endpoints to write objects to")
	pflag.String("accessKeyID", string(confDefaults["accessKeyID"]), "AccessKeyID (username) for S3 endpoints")
	pflag.String("secretAccessKey", confDefaults["secretAccessKey"], "SecretAccessKey (password) for S3 enpoints")
	pflag.String("bucket", confDefaults["bucket"], "Name of the bucket that all files should be written to")
	t, _ := strconv.ParseFloat(confDefaults["timerInterval"], 64)
	pflag.Float64("timerInterval", t, "Numbers of seconds between status messages. Use zero or negative value to turn off status updates")
	pflag.Bool("debug", false, "Output detailed information for debugging")
	pflag.Bool("ssl", false, "Use ssl for endpoint connection")
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)
	rootDir = viper.GetString("rootDir")
	maxWorkers = viper.GetInt("maxWorkers")
	endpoints = viper.GetStringSlice("endpoints")
	accessKeyID = viper.GetString("accessKeyID")
	secretAccessKey = viper.GetString("secretAccessKey")
	bucket = viper.GetString("bucket")
	timerInterval = viper.GetFloat64("timerInterval")
	debug = viper.GetBool("debug")
	ssl = viper.GetBool("ssl")
	if maxWorkers < 1 {
		log.Fatalf("maxWorkers value bad: %d", maxWorkers)
	}
	if len(endpoints) < 1 {
		log.Fatalf("No endpoints set")
	}
	if len(accessKeyID) < 1 {
		log.Fatalf("accessKeyID not set")
	}
	if len(secretAccessKey) < 1 {
		log.Fatalf("secretAccessKey not set")
	}
	if len(bucket) < 1 {
		log.Fatalf("bucket name not set")
	}

}

func main() {
	var wg sync.WaitGroup

	readConfig()

	if len(pflag.Args()) < 1 {
		fmt.Println("Missing parameter, provide file name!")
		return
	}

	nodeStats := make(chan CopyResult, maxWorkers)
	acDone := make(chan bool)
	go accumulateResults(nodeStats, acDone)

	// Setup the input queue of files to be pushed to S3 service
	files := make(chan string, maxWorkers)

	fmt.Printf("Spawning workers:")
	for worker := 0; worker < maxWorkers; worker++ {
		wg.Add(1)
		endpoint := endpoints[worker%len(endpoints)]
		if debug {
			fmt.Printf(" %d %s", worker, endpoint)
		} else {
			fmt.Printf(" %d", worker)
		}
		go copyWorker(bucket, endpoint, accessKeyID, secretAccessKey, ssl, files, nodeStats, &wg)
	}
	fmt.Printf("\n")

	// Start pushing file paths into the file queue so that workers start processing
	go fileList(pflag.Arg(0), files)

	// Setup a ticker that wakes up periodically to display status
	statusTicker := time.NewTicker(time.Duration(timerInterval * float64(time.Second)))
	go intervalStats(statusTicker.C)

	// Wait for the workers to wrap up, then wait for accumulator to finish then print final stats
	wg.Wait()
	close(nodeStats)
	statusTicker.Stop()
	<-acDone
	printStats()
}
