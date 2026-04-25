// Package meta 持有构建时注入的版本元数据。
//
// 通过 `go build -ldflags "-X github.com/wmango/tesla-cli/internal/meta.Version=..."`
// 注入,默认值见 `default*` 常量。
package meta

// 默认值:未通过 ldflags 注入时使用。
const (
	defaultVersion   = "0.0.0-dev"
	defaultCommit    = "unknown"
	defaultBuildDate = "unknown"
)

// 由 ldflags 注入的可变变量。注意:
// 不要在运行时修改这些变量;它们应被视为常量。
var (
	Version   = defaultVersion
	Commit    = defaultCommit
	BuildDate = defaultBuildDate
)

// Info 返回不可变的版本信息快照。
type Info struct {
	Version   string `json:"version"    yaml:"version"`
	Commit    string `json:"commit"     yaml:"commit"`
	BuildDate string `json:"build_date" yaml:"build_date"`
}

// Snapshot 返回当前版本元数据的副本。
func Snapshot() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
	}
}
