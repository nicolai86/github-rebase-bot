package processors

type Repository struct {
	Owner    string
	Name     string
	Mainline string
	Cache    WorkerCache
}
