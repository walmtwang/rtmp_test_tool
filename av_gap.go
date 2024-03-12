package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	flv "github.com/zhangpeihao/goflv"
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
var audioFrameIndexArray arrayFlags
var audioIntervalArray arrayFlags
var audioPackageNumArray arrayFlags
var videoFrameIndexArray arrayFlags
var videoIntervalArray arrayFlags
var videoPackageNumArray arrayFlags

var obConn rtmp.OutboundConn
var createStreamChan chan rtmp.OutboundStream
var videoDataSize int64
var audioDataSize int64
var flvFile *flv.File

var audioTags = make(chan flvTag, 100000)
var videoTags = make(chan flvTag, 100000)

type flvTag struct {
	header *flv.TagHeader
	data   []byte
}

var audioCurrentTime int64 = 0
var videoCurrentTime int64 = 0

var status uint

func getAudioInterval() int64 {
	var interval int64 = 0
	for i := 0; i < len(audioFrameIndexArray); i++ {
		if audioCurrentTime >= audioFrameIndexArray[i] {
			interval = audioIntervalArray[i]
		} else {
			return interval
		}
	}
	return interval
}
func getAudioPackageNum() int64 {
	var packageNum int64 = 0
	for i := 0; i < len(audioFrameIndexArray); i++ {
		if audioCurrentTime >= audioFrameIndexArray[i] {
			packageNum = audioPackageNumArray[i]
		} else {
			return packageNum
		}
	}
	return packageNum
}

func getVideoInterval() int64 {
	var interval int64 = 0
	for i := 0; i < len(videoFrameIndexArray); i++ {
		if videoCurrentTime >= videoFrameIndexArray[i] {
			interval = videoIntervalArray[i]
		} else {
			return interval
		}
	}
	return interval
}
func getVideoPackageNum() int64 {
	var packageNum int64 = 0
	for i := 0; i < len(videoFrameIndexArray); i++ {
		if videoCurrentTime >= videoFrameIndexArray[i] {
			packageNum = videoPackageNumArray[i]
		} else {
			return packageNum
		}
	}
	return packageNum
}

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

func ReadTag(flvFile *flv.File) (*flv.TagHeader, []byte, error) {
	header, data, err := flvFile.ReadTag()
	if err != nil {
		return nil, nil, err
	}

	if header.TagType == flv.AUDIO_TAG {
		audioCurrentTime++
		audioTags <- flvTag{
			header: header,
			data:   data,
		}
		audioTagLength := getAudioPackageNum() + 1
		if len(audioTags) < int(audioTagLength) {
			return nil, nil, nil
		}
		for len(audioTags) > int(audioTagLength) {
			<-audioTags
		}
		audioTag := <-audioTags
		return audioTag.header, audioTag.data, nil
	} else if header.TagType == flv.VIDEO_TAG {
		videoCurrentTime++
		videoTags <- flvTag{
			header: header,
			data:   data,
		}
		videoTagLength := getVideoPackageNum() + 1
		if len(videoTags) < int(videoTagLength) {
			return nil, nil, nil
		}
		for len(videoTags) > int(videoTagLength) {
			<-videoTags
		}
		videoTag := <-videoTags
		return videoTag.header, videoTag.data, nil

	} else {
		return header, data, nil
	}

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
	startTs := uint32(0)
	startAt := time.Now().UnixNano()
	//preTs := uint32(0)
	fmt.Println("3")
	for status == rtmp.OUTBOUND_CONN_STATUS_CREATE_STREAM_OK {
		if flvFile.IsFinished() {
			fmt.Println("@@@@@@@@@@@@@@File finished")
			flvFile.Close()
			tmpFile, err := flv.OpenFile(flvFilePath)
			if err != nil {
				fmt.Println("Open FLV dump file error:", err)
				return
			}
			flvFile = tmpFile
			//flvFile.LoopBack()
			startTs = uint32(0)
			startAt = time.Now().UnixNano()

			audioTags = make(chan flvTag, 100000)
			videoTags = make(chan flvTag, 100000)

			audioCurrentTime = 0
			videoCurrentTime = 0
		}
		header, data, err := ReadTag(flvFile)
		if err != nil {
			fmt.Println("flvFile.ReadTag() error:", err)
			break
		}
		if header == nil {
			continue
		}
		switch header.TagType {
		case flv.VIDEO_TAG:
			videoDataSize += int64(len(data))
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

		diff2 := uint32((time.Now().UnixNano() - startAt) / 1000000)
		//		fmt.Printf("diff1: %d, diff2: %d\n", diff1, diff2)
		if diff1 > diff2+100 {
			//			fmt.Printf("header.Timestamp: %d, now: %d\n", header.Timestamp, time.Now().UnixNano())
			time.Sleep(time.Millisecond * time.Duration(diff1-diff2))
		}

		interval := int64(0)
		if header.TagType == flv.AUDIO_TAG {
			interval = getAudioInterval()
		}
		if header.TagType == flv.VIDEO_TAG {
			interval = getVideoInterval()
		}
		deltaTimestamp = uint32(int64(deltaTimestamp) + interval)
		//fmt.Println("deltaTimestamp:", deltaTimestamp)

		//fmt.Printf("@@@@@@@@@@@@@@diff1 header(%+v), startTs: %d\n", header, startTs)
		//if err = stream.PublishData(header.TagType, data, diff1); err != nil {
		if err = stream.PublishData(header.TagType, data, deltaTimestamp); err != nil {
			fmt.Println("PublishData() error:", err)
			break
		}
	}
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s version[%s]\r\nUsage: %s [OPTIONS]\r\n", programName, version, os.Args[0])
		flag.PrintDefaults()
	}
	flag.StringVar(&tcUrl, "tcUrl", "", "tcUrl")
	flag.StringVar(&streamName, "streamName", "", "streamName")
	flag.StringVar(&flvFilePath, "flvFile", "", "flvFile")
	flag.Var(&audioFrameIndexArray, "audioFrameIndex", "audioFrameIndex, You can set multiple")
	flag.Var(&audioIntervalArray, "audioInterval", "audioInterval, You can set multiple")
	flag.Var(&audioPackageNumArray, "audioPackageNum", "audioPackageNum, You can set multiple")
	flag.Var(&videoFrameIndexArray, "videoFrameIndex", "videoFrameIndex, You can set multiple")
	flag.Var(&videoIntervalArray, "videoInterval", "videoInterval, You can set multiple")
	flag.Var(&videoPackageNumArray, "videoPackageNum", "videoPackageNum, You can set multiple")

	flag.Parse()

	if tcUrl == "" || streamName == "" || flvFilePath == "" ||
		len(audioFrameIndexArray) == 0 || len(audioIntervalArray) == 0 || len(audioPackageNumArray) == 0 ||
		len(videoFrameIndexArray) == 0 || len(videoIntervalArray) == 0 || len(videoPackageNumArray) == 0 {
		flag.PrintDefaults()
		return
	}

	if len(audioFrameIndexArray) != len(audioIntervalArray) {
		fmt.Println("len(audioFrameIndexArray) != len(audioIntervalArray)")
		return
	}

	if len(audioFrameIndexArray) != len(audioPackageNumArray) {
		fmt.Println("len(audioFrameIndexArray) != len(audioPackageNumArray)")
		return
	}

	if len(videoFrameIndexArray) != len(videoIntervalArray) {
		fmt.Println("len(videoFrameIndexArray) != len(videoIntervalArray)")
		return
	}

	if len(videoFrameIndexArray) != len(videoPackageNumArray) {
		fmt.Println("len(videoFrameIndexArray) != len(videoPackageNumArray)")
		return
	}

	fmt.Println("audioFrameIndexArray")
	fmt.Println(audioFrameIndexArray)
	fmt.Println("audioIntervalArray")
	fmt.Println(audioIntervalArray)
	fmt.Println("audioPackageNumArray")
	fmt.Println(audioPackageNumArray)
	fmt.Println("videoFrameIndexArray")
	fmt.Println(videoIntervalArray)
	fmt.Println("videoIntervalArray")
	fmt.Println(videoIntervalArray)
	fmt.Println("videoFrameIndexArray")
	fmt.Println(videoFrameIndexArray)

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
