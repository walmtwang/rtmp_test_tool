package main

/*
import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/zhangpeihao/goflv"
	rtmp "rtmp_test_tool/gortmp"
)

const (
	programName = "RtmpPublisher"
	version     = "0.0.1"
)

type arrayFlags []int64

// Value ...
func (i *arrayFlags) String() string {
	return fmt.Sprint(*i)
}

// Set 方法是flag.Value接口, 设置flag Value的方法.
// 通过多个flag指定的值， 所以我们追加到最终的数组上.
func (i *arrayFlags) Set(value string) error {
	intData, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("strconv.ParseInt failed, err:%v", err)
	}
	*i = append(*i, intData)
	return nil
}

type TestOutboundConnHandler struct {
}

var tcUrl string
var streamName string
var flvFilePath string
var frameIndexArray arrayFlags
var intervalArray arrayFlags

var obConn rtmp.OutboundConn
var createStreamChan chan rtmp.OutboundStream
var videoDataSize int64
var audioDataSize int64
var flvFile *flv.File

var status uint

func (handler *TestOutboundConnHandler) OnStatus(conn rtmp.OutboundConn) {
	var err error
	if obConn == nil {
		return
	}
	status, err = obConn.Status()
	fmt.Printf("@@@@@@@@@@@@@status: %d, err: %v\n", status, err)
}

func (handler *TestOutboundConnHandler) OnClosed(conn rtmp.Conn) {
	fmt.Printf("@@@@@@@@@@@@@Closed\n")
}

func (handler *TestOutboundConnHandler) OnReceived(conn rtmp.Conn, message *rtmp.Message) {
}

func (handler *TestOutboundConnHandler) OnReceivedRtmpCommand(conn rtmp.Conn, command *rtmp.Command) {
	fmt.Printf("ReceviedRtmpCommand: %+v\n", command)
}

func (handler *TestOutboundConnHandler) OnStreamCreated(conn rtmp.OutboundConn, stream rtmp.OutboundStream) {
	fmt.Printf("Stream created: %d\n", stream.ID())
	createStreamChan <- stream
}
func (handler *TestOutboundConnHandler) OnPlayStart(stream rtmp.OutboundStream) {

}
func (handler *TestOutboundConnHandler) OnPublishStart(stream rtmp.OutboundStream) {
	// Set chunk buffer size
	go publish(stream)
}

func publish(stream rtmp.OutboundStream) {
	fmt.Println("1")
	var err error
	flvFile, err = flv.OpenFile(flvFilePath)
	if err != nil {
		fmt.Println("Open FLV dump file error:", err)
		return
	}
	fmt.Println("2")
	defer flvFile.Close()
	startTs := uint32(0)
	startAt := time.Now().UnixNano()
	var currentTime int64 = 0
	//preTs := uint32(0)
	fmt.Println("3")
	for status == rtmp.OUTBOUND_CONN_STATUS_CREATE_STREAM_OK {
		if flvFile.IsFinished() {
			fmt.Println("@@@@@@@@@@@@@@File finished")
			break
			//flvFile.LoopBack()
			//startAt = time.Now().UnixNano()
			//startTs = uint32(0)
			//currentTime := 0
			//preTs = uint32(0)
		}
		header, data, err := flvFile.ReadTag()
		if err != nil {
			fmt.Println("flvFile.ReadTag() error:", err)
			break
		}
		switch header.TagType {
		case flv.VIDEO_TAG:
			videoDataSize += int64(len(data))
			currentTime++
		case flv.AUDIO_TAG:
			audioDataSize += int64(len(data))
		}

		if startTs == uint32(0) {
			startTs = header.Timestamp
		}
		diff1 := uint32(0)
		//		deltaTs := uint32(0)
		if header.Timestamp > startTs {
			diff1 = header.Timestamp - startTs
		} else {
			//fmt.Printf("@@@@@@@@@@@@@@diff1 header(%+v), startTs: %d\n", header, startTs)
		}
		//if diff1 > preTs {
		//	//			deltaTs = diff1 - preTs
		//	preTs = diff1
		//}

		deltaTimestamp := diff1
		interval := calculateInterval(currentTime)
		deltaTimestamp = uint32(int64(deltaTimestamp) + interval)
		//fmt.Println("deltaTimestamp:", deltaTimestamp)

		//fmt.Printf("@@@@@@@@@@@@@@diff1 header(%+v), startTs: %d\n", header, startTs)
		//if err = stream.PublishData(header.TagType, data, diff1); err != nil {
		if err = stream.PublishData(header.TagType, data, deltaTimestamp); err != nil {
			fmt.Println("PublishData() error:", err)
			break
		}
		diff2 := uint32((time.Now().UnixNano() - startAt) / 1000000)
		//		fmt.Printf("diff1: %d, diff2: %d\n", diff1, diff2)
		if diff1 > diff2+100 {
			//			fmt.Printf("header.Timestamp: %d, now: %d\n", header.Timestamp, time.Now().UnixNano())
			time.Sleep(time.Millisecond * time.Duration(diff1-diff2))
		}
	}
}

func calculateInterval(currentTimes int64) int64 {
	var interval int64 = 0
	for i := 0; i < len(frameIndexArray); i++ {
		if currentTimes >= frameIndexArray[i] {
			interval = intervalArray[i]
		}
	}
	return interval
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s version[%s]\r\nUsage: %s [OPTIONS]\r\n", programName, version, os.Args[0])
		flag.PrintDefaults()
	}
	flag.StringVar(&tcUrl, "tcUrl", "", "tcUrl")
	flag.StringVar(&streamName, "streamName", "", "streamName")
	flag.StringVar(&flvFilePath, "flvFile", "", "flvFile")
	flag.Var(&frameIndexArray, "frameIndex", "frameIndex, You can set multiple")
	flag.Var(&intervalArray, "interval", "interval, You can set multiple")

	flag.Parse()

	if tcUrl == "" || streamName == "" || flvFilePath == "" || len(
		frameIndexArray) == 0 || len(intervalArray) == 0 {
		flag.PrintDefaults()
		return
	}

	if len(frameIndexArray) != len(intervalArray) {
		fmt.Println("len(frameIndexArray) != len(intervalArray)")
		return
	}

	//l := log.NewLogger(".", "publisher", nil, 60, 3600*24, true)
	//rtmp.InitLogger(l)
	//defer l.Close()
	createStreamChan = make(chan rtmp.OutboundStream)
	testHandler := &TestOutboundConnHandler{}
	fmt.Println("to dial")
	fmt.Println("a")
	var err error
	obConn, err = rtmp.Dial(tcUrl, testHandler, 100)
	if err != nil {
		fmt.Println("Dial error", err)
		os.Exit(-1)
	}
	fmt.Println("b")
	defer obConn.Close()
	fmt.Println("to connect")
	err = obConn.Connect()
	if err != nil {
		fmt.Printf("Connect error: %s", err.Error())
		os.Exit(-1)
	}
	fmt.Println("c")
	for {
		select {
		case stream := <-createStreamChan:
			// Publish
			stream.Attach(testHandler)
			err = stream.Publish(streamName, "live")
			if err != nil {
				fmt.Printf("Publish error: %s", err.Error())
				os.Exit(-1)
			}

		case <-time.After(1 * time.Second):
			fmt.Printf("Audio size: %d bytes; Vedio size: %d bytes\n", audioDataSize, videoDataSize)
		}
	}
}
*/
