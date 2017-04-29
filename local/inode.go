package local

var (
	inodeCh = make(chan uint64)
)

func init() {
	globalInode := uint64(10000) // lower used for misc stuff
	go func() {
		for {
			globalInode++
			inodeCh <- globalInode
		}
	}()
}
