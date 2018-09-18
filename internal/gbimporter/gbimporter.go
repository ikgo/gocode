package gbimporter

import (
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

// We need to mangle go/build.Default to make gcimporter work as
// intended, so use a lock to protect against concurrent accesses.
var buildDefaultLock sync.Mutex

type pkgCache struct {
	cacheLock   *sync.Mutex
	pkgCache    map[string][]*packages.Package
	pkgErrCache map[string]error
}

var pc *pkgCache

func init() {
	pc = &pkgCache{
		cacheLock: &sync.Mutex{},
		pkgCache: make(map[string][]*packages.Package),
		pkgErrCache: make(map[string]error),
	}
}
// importer implements types.ImporterFrom and provides transparent
// support for gb-based projects.
type importer struct {
	underlying types.ImporterFrom
	ctx        *PackedContext
	gbroot     string
	gbpaths    []string
}

func New(ctx *PackedContext, filename string, underlying types.ImporterFrom) types.ImporterFrom {
	imp := &importer{
		ctx:        ctx,
		underlying: underlying,
	}

	slashed := filepath.ToSlash(filename)
	i := strings.LastIndex(slashed, "/vendor/src/")
	if i < 0 {
		i = strings.LastIndex(slashed, "/src/")
	}
	if i > 0 {
		paths := filepath.SplitList(imp.ctx.GOPATH)

		gbroot := filepath.FromSlash(slashed[:i])
		gbvendor := filepath.Join(gbroot, "vendor")
		if samePath(gbroot, imp.ctx.GOROOT) {
			goto Found
		}
		for _, path := range paths {
			if samePath(path, gbroot) || samePath(path, gbvendor) {
				goto Found
			}
		}

		imp.gbroot = gbroot
		imp.gbpaths = append(paths, gbroot, gbvendor)
	Found:
	}

	return imp
}

func (i *importer) Import(path string) (*types.Package, error) {
	return i.ImportFrom(path, "", 0)
}

func (pc *pkgCache) Load(srcDir string, src ...string) ([]*packages.Package, error) {
	keyList := []string{srcDir}
	keyList = append(keyList, src...)
	key := strings.Join(keyList, string(os.PathListSeparator))
	pc.cacheLock.Lock()
	defer pc.cacheLock.Unlock()
	val, exists := pc.pkgCache[key]
	if exists {
		return val,pc.pkgErrCache[key]
	}
	val, err := packages.Load(&packages.Config{
		Mode: packages.LoadTypes,
		Dir:  srcDir,
	}, src...)
	pc.pkgCache[key] = val
	pc.pkgErrCache[key] = err
	return val,err
}

func (i *importer) ImportFrom(path, srcDir string, mode types.ImportMode) (*types.Package, error) {
	buildDefaultLock.Lock()
	defer buildDefaultLock.Unlock()

	var src []string
	src = append(src, path)
	pkgs, err := pc.Load(srcDir, src...)

	// origDef := build.Default
	// defer func() { build.Default = origDef }()

	// def := &build.Default
	// def.GOARCH = i.ctx.GOARCH
	// def.GOOS = i.ctx.GOOS
	// def.GOROOT = i.ctx.GOROOT
	// def.GOPATH = i.ctx.GOPATH
	// def.CgoEnabled = i.ctx.CgoEnabled
	// def.UseAllFiles = i.ctx.UseAllFiles
	// def.Compiler = i.ctx.Compiler
	// def.BuildTags = i.ctx.BuildTags
	// def.ReleaseTags = i.ctx.ReleaseTags
	// def.InstallSuffix = i.ctx.InstallSuffix

	// def.SplitPathList = i.splitPathList
	// def.JoinPath = i.joinPath

	// var result, err = i.underlying.ImportFrom(path, srcDir, mode)
	if len(pkgs) > 0 {
		return pkgs[0].Types, nil
	}
	return nil, err
}

func (i *importer) splitPathList(list string) []string {
	if i.gbroot != "" {
		return i.gbpaths
	}
	return filepath.SplitList(list)
}

func (i *importer) joinPath(elem ...string) string {
	res := filepath.Join(elem...)

	if i.gbroot != "" {
		// Want to rewrite "$GBROOT/(vendor/)?pkg/$GOOS_$GOARCH(_)?"
		// into "$GBROOT/pkg/$GOOS-$GOARCH(-)?".
		// Note: gb doesn't use vendor/pkg.
		if gbrel, err := filepath.Rel(i.gbroot, res); err == nil {
			gbrel = filepath.ToSlash(gbrel)
			gbrel, _ = match(gbrel, "vendor/")
			if gbrel, ok := match(gbrel, fmt.Sprintf("pkg/%s_%s", i.ctx.GOOS, i.ctx.GOARCH)); ok {
				gbrel, hasSuffix := match(gbrel, "_")

				// Reassemble into result.
				if hasSuffix {
					gbrel = "-" + gbrel
				}
				gbrel = fmt.Sprintf("pkg/%s-%s/", i.ctx.GOOS, i.ctx.GOARCH) + gbrel
				gbrel = filepath.FromSlash(gbrel)
				res = filepath.Join(i.gbroot, gbrel)
			}
		}
	}

	return res
}

func match(s, prefix string) (string, bool) {
	rest := strings.TrimPrefix(s, prefix)
	return rest, len(rest) < len(s)
}
