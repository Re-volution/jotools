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
	var exS = ""
	if f.day == now.Day() {
		if f.size > maxsize {
			f.count++
			exS = strconv.Itoa(f.count)
		}
	} else {
		f.count = 0
	}

	if f.w != nil {
		f.w.Flush()
		f.h.Close()
	}
	fname := ""
	if f.count != 0 {
		fname = pathdir + f.name + "/" + now.Format("2006_01_02") + "_" + exS + ".log"
	} else {
		fname = pathdir + f.name + "/" + now.Format("2006_01_02") + ".log"
	}
	h, e := os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if e != nil {
		fmt.Println("创建文件失败:", fname, e)
		return
	}
	f.day = now.Day()
	w := bufio.NewWriterSize(h, 1024*256)
	f.w, f.h = w, h
	f.size = 0
}

func checkErrFile() {
	now := ltime.GetNow()
	fname := pathdir + "errorCode/" + f.name + now.Format("2006_01_02") + ".log"
	if fname == f.fname {
		return
	}
	f.fname = fname
	h, e := os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if e != nil {
		fmt.Println("创建文件失败:", fname, e)
		return
	}
	f.errF = h
}

func Close() {
	f.Lock()
	f.w.Flush()
	f.Unlock()
}

func init() {
	var err error
	f = new(File)
	f.ch = make(chan string, 1024)
	f.errCh = make(chan string, 1024)
	if _, err = os.Stat(pathdir); os.IsNotExist(err) {
		os.Mkdir(pathdir, 0755)
	}

	f.name = path.Base(filepath.ToSlash(os.Args[0]))
	if _, err = os.Stat(pathdir + f.name + "/"); os.IsNotExist(err) {
		os.Mkdir(pathdir+f.name+"/", 0755)
	}

	if _, err = os.Stat(pathdir + "errorCode/"); os.IsNotExist(err) {
		os.Mkdir(pathdir+"errorCode/", 0755)
	}

	checkErrFile()
	checkFile()

	go run()
	go flush()
}

func flush() {
	for {
		time.Sleep(30 * time.Second)
		f.Lock()
		checkFile()
		if f.w != nil && f.w.Size() > 0 {
			f.w.Flush()
		} else {
			return
		}
		f.Unlock()
	}

}

func run() {
	i := 0
	go func() {
		for {
			select {
			case data := <-f.errCh:
				checkErrFile()
				f.errF.Write([]byte(data))
			default:
				time.Sleep(time.Second)
			}
		}
	}()
	for {
		i = 0
		f.Lock()
		for {
			i++
			select {
			case data := <-f.ch:
				f.w.Write([]byte(data))
				f.size += int64(len(data))
			default:
				i = 51
				time.Sleep(time.Millisecond * 20)
			}
			if i > 50 {
				break
			}
		}
		checkFile()
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
	if !nodebug {
		fmt.Print(getNow().Format(time.DateTime) + ": " + fmt.Sprint(data) + "\r\n")
	}
}

func Warring(data ...any) {
	warring("warring:" + fmt.Sprint(data))
}

func Error(data ...any) {
	_, fill, line, _ := runtime.Caller(1)
	errLog(fmt.Sprint("Err: 文件:", fill, " 行:", line, ":", data))
}

func Error2(data ...any) {
	_, fill, line, _ := runtime.Caller(2)
	errLog(fmt.Sprint("Err: 文件:", fill, " 行:", line, ":", data))
}

func Errorf(data ...any) {
	ds := ""
	if len(data) > 0 {
		fo, ok := data[0].(string)
		if ok {
			ds = fmt.Sprintf(fo, data[1:]...)
		}
	}
	_, fill, line, _ := runtime.Caller(1)
	errLog(fmt.Sprint("Err: 文件:", fill, " 行:", line, ":", ds))
}

func Log(a ...any) {
	log(fmt.Sprint(a))
}
