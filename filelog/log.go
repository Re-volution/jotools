package filelog

import (
	"bufio"
	"fmt"
	"github.com/Re-volution/ltime"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type File struct {
	h *os.File
	w *bufio.Writer
	errFile
	size int64
	name string
	ch   chan string
	sync.Mutex
}

type errFile struct {
	errF  *os.File
	errCh chan string
	fname string
}

const maxsize = 512 * 1024

var nodebug = false

var f *File
var pathdir = "../log/"

func checkFile() {
	if f.h != nil && f.size < maxsize {
		return
	}
	if f.w != nil {
		f.w.Flush()
		f.h.Close()
	}
	now := time.Now()

	fname := pathdir + f.name + now.Format("2006_01_02_15_04_05") + ".log"
	h, e := os.Create(fname)
	if e != nil {
		fmt.Println("创建文件失败:", fname, e)
		return
	}
	w := bufio.NewWriterSize(h, 1024)
	f.w, f.h = w, h
	f.size = 0
}

func checkErrFile() {
	now := ltime.GetNow()
	fname := pathdir + "error." + f.name + now.Format("2006_01_02") + ".log"
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
	time.Sleep(2e9)
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
	checkErrFile()
	checkFile()

	go run()
	go flush()
}

func flush() {
	for {
		time.Sleep(30 * time.Second)
		f.Lock()
		if f.w != nil {
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
				time.Sleep(time.Second)
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
	Debug("-------log:", data)
	writeChan(f.ch, getNow().Format(time.DateTime)+": "+data+"\r\n")
}

func errLog(data string) {
	wd := getNow().Format(time.DateTime) + ": " + data + "\r\n"
	fmt.Print(wd)
	writeChan(f.errCh, wd)
}

func Debug(data ...any) {
	if !nodebug {
		fmt.Print(getNow().Format(time.DateTime) + ": " + fmt.Sprint(data) + "\r\n")
	}
}

func Warring(data ...any) {
	Log("warring:", data)
}

func Error(data ...any) {
	_, fill, line, _ := runtime.Caller(1)
	errLog(fmt.Sprint(" 文件:", fill, " 行:", line, ":", data))
}

func Log(a ...any) {
	log(fmt.Sprint(a))
}
