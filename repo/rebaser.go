package repo

import (
	"context"
	"log"
)

type branchRebaser struct {
	w     GitWorker
	cache GitCache
	queue chan chan Signal
	ctx   context.Context
}

func (b *branchRebaser) run() {
	for {
		select {
		case ch := <-b.queue:
			func(ch chan Signal) {
				removeWorktreeBranch(b.cache.cacheDirectory(), b.w.Branch())

				dir, err := b.w.prepare()
				if err != nil {
					ch <- Signal{Error: err}
					close(ch)
					return
				}

				if err := b.w.update(dir); err != nil {
					log.Printf("failed to update worktree: %v", err)
					ch <- Signal{Error: err}
					close(ch)
					return
				}

				log.Printf("rebasing…")
				up2date, err := b.w.rebase(dir)
				if err != nil {
					log.Printf("failed to rebase mainline: %v", err)
					ch <- Signal{Error: err}
					close(ch)
					return
				}

				if !up2date {
					if err := b.w.push(dir); err != nil {
						log.Printf("failed to push branch: %v", err)
						ch <- Signal{Error: err}
						close(ch)
						return
					}

					ch <- Signal{Error: nil, UpToDate: false}
					close(ch)
					return
				}

				ch <- Signal{Error: err, UpToDate: true}
				close(ch)
			}(ch)
		case <-b.ctx.Done():
			b.cache.Cleanup(b.w)
			return
		}
	}
}
