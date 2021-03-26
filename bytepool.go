package bytepool

import (
	"context"
	"runtime"
	"sync"
	"time"
	"unsafe"
)

// NewBytePool 生成的pool会限制最多分配 maxCount*bufferSize 大小内存，
// 超过之后会阻止继续分配内存防止程序oom
func NewBytePool(maxCount, bufferSize int, gcDuration time.Duration) *BytePool {
	ch := make(chan []byte, maxCount)
	for i := 0; i < maxCount; i++ {
		ch <- nil
	}
	bp := &BytePool{ch: ch, size: bufferSize}
	if gcDuration > 0 {
		go bp.loopGC(gcDuration)
	}
	return bp
}

type BytePool struct {
	ch   chan []byte
	size int
	all  sync.Map
}

// Get 在全部分配完的情况下会一直等待直到拿到数据
func (bp *BytePool) Get() []byte {
	buf, _ := bp.GetWithCtx(context.Background())
	return buf
}

// GetWithCtx 可以通过context的超时来控制不再等待提前返回
func (bp *BytePool) GetWithCtx(ctx context.Context) (b []byte, err error) {
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case b = <-bp.ch:
		if b == nil {
			b = make([]byte, bp.size)
			// 如果用户忘记把数据put回来，这里put一个nil进去，
			// 保证channel里面的元素个数不减少
			runtime.SetFinalizer(&b, func(b *[]byte) {
				bp.Put(nil)
			})
		} else {
			bp.all.Delete(uintptr(unsafe.Pointer(&b)))
		}
	}
	return
}

func (bp *BytePool) Put(p []byte) {
	if cap(p) != bp.size {
		p = nil
	} else {
		p = p[:bp.size]
		// 检测p是否重复放回，防止重复分配导致数据错乱
		_, loaded := bp.all.LoadOrStore(
			uintptr(unsafe.Pointer(&p)), nil)
		if loaded {
			return
		}
	}
	select {
	case bp.ch <- p:
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
		case b := <-bp.ch:
			if b != nil {
				// 主动清理的元素清除原来设置的runtime.SetFinalizer，
				// 防止重复放回
				runtime.SetFinalizer(&b, nil)
			}
			bp.Put(nil)
		default:
		}
	}
}
