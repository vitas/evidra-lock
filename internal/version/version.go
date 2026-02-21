package version

var (
	Version = "dev"
	Commit  = "dev"
	Date    = "dev"
)

func String() string {
	return Version
}
