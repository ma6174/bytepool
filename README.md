# BytePool

参考 https://github.com/oxtoacart/bpool，但是增加了如下功能

- 控制总的内存使用量，超过之后会等待，防止OOM
- 为了防止超内存之后Get超时，增加了context来控制超时时间，防止死锁
- 增加GC功能，定期清理内存，闲时释放内存
- 如果忘记Put会在GC的时候自动Put回去
- 自动防止重复Put，避免数据错乱
