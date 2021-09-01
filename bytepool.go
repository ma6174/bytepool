package bytepool

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// NewBytePool 生成的pool会限制最多分配 maxCount*bufferSize 大小内存，
// 超过之后会阻止继续分配内存防止程序oom
func NewBytePool(maxCount, bufferSize int, gcDuration time.Duration) *BytePool {
	ch := make(chan *Bs, maxCount)
	for i := 0; i < maxCount; i++ {
		ch <- nil
	}
	bp := &BytePool{ch: ch, size: bufferSize}
	if gcDuration > 0 {
		go bp.loopGC(gcDuration)
	}
	return bp
}

var idx uint64

func getNextIdx() uint64 {
	return atomic.AddUint64(&idx, 1)
}

type Bs struct {
	idx uint64
	bs  []byte
	bp  *BytePool
}

func (d *Bs) Data() []byte {
	return d.bs
}

func (d *Bs) PutBack() {
	d.bp.put(d)
}

type BytePool struct {
	ch   chan *Bs
	size int
	all  sync.Map
}

// Get 在全部分配完的情况下会一直等待直到拿到数据
func (bp *BytePool) Get() *Bs {
	bs, _ := bp.GetWithCtx(context.Background())
	return bs
}

// GetWithCtx 可以通过context的超时来控制不再等待提前返回
func (bp *BytePool) GetWithCtx(ctx context.Context) (bs *Bs, err error) {
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case bs = <-bp.ch:
		if bs == nil {
			bs = &Bs{
				idx: getNextIdx(),
				bs:  make([]byte, bp.size),
				bp:  bp,
			}
			// 如果用户忘记把数据put回来，这里put一个nil进去，
			// 保证channel里面的元素个数不减少
			runtime.SetFinalizer(bs, func(bs *Bs) {
				bs.PutBack()
			})
		} else {
			bp.all.Delete(bs.idx)
		}
	}
	return
}

func (bp *BytePool) put(bs *Bs) {
	if bs != nil {
		// 检测p是否重复放回，防止重复分配导致数据错乱
		_, loaded := bp.all.LoadOrStore(bs.idx, nil)
		if loaded {
			return
		}
	}
	select {
	case bp.ch <- bs:
	default:
	}
}

func (bp *BytePool) BufferSize() int {
	return bp.size
}

func (bp *BytePool) loopGC(gcDuration time.Duration) {
	for range time.Tick(gcDuration) {
		// 强制执行一次GC，让 runtime.SetFinalizer 里面的函数执行
		runtime.GC()
		select {
		case bs := <-bp.ch:
			if bs != nil {
				bs.bs = nil
				// 主动清理的元素清除原来设置的runtime.SetFinalizer，
				// 防止重复放回
				runtime.SetFinalizer(bs, nil)
			}
			bp.put(nil)
		default:
		}
	}
}
