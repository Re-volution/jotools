package filelog

import (
	"bufio"
	"fmt"
	"github.com/Re-volution/ltime"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"
)

type File struct {
	h *os.File
	w *bufio.Writer
	errFile
	size  int64
	name  string
	day   int
	ch    chan string
	count int
	sync.Mutex
}

type errFile struct {
	errF  *os.File
	errCh chan string
	fname string
}

const maxsize = 1024 * 1024

var nodebug = false

var f *File
var pathdir = "./log/"

func checkFile() {
	now := ltime.GetNow()
	if f.h != nil && f.size < maxsize && f.day == now.Day() {
		return
	}
	if f.h != nil && f.day == now.Day() && f.size >= maxsize {
		f.count++
	} else if f.h != nil && f.day != now.Day() {
		f.count = 0
	}
	fname := pathdir + f.name + "/" + now.Format("2006_01_02") + ".log"
	if f.count != 0 {
		fname = pathdir + f.name + "/" + now.Format("2006_01_02") + "_" + strconv.Itoa(f.count) + ".log"
	}
	h, err := os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		fmt.Println("创建文件失败:", fname, err)
		return
	}
	if f.w != nil {
		_ = f.w.Flush()
	}
	if f.h != nil && f.h != h {
		_ = f.h.Close()
	}
	f.day = now.Day()
	f.w = bufio.NewWriterSize(h, 1024*256)
	f.h = h
	f.size = 0
	if info, statErr := h.Stat(); statErr == nil {
		f.size = info.Size()
	}
}

func checkErrFile() {
	now := ltime.GetNow()
	fname := pathdir + "errorCode/" + f.name + now.Format("2006_01_02") + ".log"
	if fname == f.fname && f.errF != nil {
		return
	}
	h, err := os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		fmt.Println("创建文件失败:", fname, err)
		return
	}
	if f.errF != nil && f.errF != h {
		_ = f.errF.Close()
	}
	f.errF = h
	f.fname = fname
}

func Close() {
	f.Lock()
	defer f.Unlock()
	if f.w != nil {
		_ = f.w.Flush()
	}
}

func init() {
	f = new(File)
	f.ch = make(chan string, 1024)
	f.errCh = make(chan string, 1024)
	if err := os.MkdirAll(pathdir, 0755); err != nil {
		fmt.Println("创建日志目录失败:", err)
	}
	f.name = path.Base(filepath.ToSlash(os.Args[0]))
	if err := os.MkdirAll(pathdir+f.name+"/", 0755); err != nil {
		fmt.Println("创建日志目录失败:", err)
	}
	if err := os.MkdirAll(pathdir+"errorCode/", 0755); err != nil {
		fmt.Println("创建日志目录失败:", err)
	}
	checkErrFile()
	checkFile()
	go run()
	go flush()
}

func flush() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		f.Lock()
		checkFile()
		if f.w != nil && f.w.Buffered() > 0 {
			_ = f.w.Flush()
		}
		f.Unlock()
	}
}

func run() {
	go errLoop()
	for {
		f.Lock()
		n := 0
		idle := false
		for n < 50 {
			select {
			case data := <-f.ch:
				if f.w != nil {
					_, _ = f.w.Write([]byte(data))
					f.size += int64(len(data))
				}
				n++
			default:
				idle = true
				n = 50
			}
		}
		checkFile()
		f.Unlock()
		if idle {
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func errLoop() {
	for data := range f.errCh {
		f.Lock()
		checkErrFile()
		if f.errF != nil {
			_, _ = f.errF.Write([]byte(data))
		}
		f.Unlock()
	}
}

func getNow() time.Time {
	return ltime.GetNow()
}

func writeChan(c chan string, d string) {
	select {
	case c <- d:
	default:
		fmt.Println("write chan fail:", d)
	}
}

func log(data string) {
	Debug(data)
	writeChan(f.ch, getNow().Format(time.DateTime)+": "+data+"\r\n")
}

func warring(data string) {
	wd := getNow().Format(time.DateTime) + ": " + data + "\r\n"
	Debug(wd)
	writeChan(f.ch, wd)
}

func errLog(data string) {
	wd := getNow().Format(time.DateTime) + ": " + data + "\r\n"
	Debug(wd)
	writeChan(f.errCh, wd)
}

func Debug(data ...any) {
	if nodebug {
		return
	}
	fmt.Print(getNow().Format(time.DateTime) + ": " + fmt.Sprint(data...) + "\r\n")
}

func Warring(data ...any) {
	warring("warring:" + fmt.Sprint(data...))
}

func Error(data ...any) {
	_, file, line, _ := runtime.Caller(1)
	errLog(fmt.Sprint("Err: 文件:", file, " 行:", line, ":", fmt.Sprint(data...)))
}

func Error2(data ...any) {
	_, file, line, _ := runtime.Caller(2)
	errLog(fmt.Sprint("Err: 文件:", file, " 行:", line, ":", fmt.Sprint(data...)))
}

func Errorf(data ...any) {
	ds := ""
	if len(data) > 0 {
		if fo, ok := data[0].(string); ok {
			ds = fmt.Sprintf(fo, data[1:]...)
		}
	}
	_, file, line, _ := runtime.Caller(1)
	errLog(fmt.Sprint("Err: 文件:", file, " 行:", line, ":", ds))
}

func Log(a ...any) {
	log(fmt.Sprint(a...))
}
