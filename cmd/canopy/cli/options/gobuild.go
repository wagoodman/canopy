package options

import (
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"

	"github.com/anchore/fangs"
)

var _ interface {
	fangs.FlagAdder
	fangs.PostLoader
} = (*GoBuild)(nil)

// GoBuild mimics the flags for `go build` as much as is feasible. The RenderedFlags field ultimately holds the rendered
// flags to be passed to the `go test` or `go build` command.
type GoBuild struct {
	All           bool
	Asan          bool
	Asmflags      string
	Buildmode     string
	Buildvcs      string
	Cd            string
	Compiler      string
	Gccgoflags    string
	Gcflags       string
	Installsuffix string
	Ldflags       string
	Linkshared    bool
	Mod           string
	Modcacherw    bool
	Modfile       string
	Msan          bool
	Overlay       string
	Pgo           string
	Pkgdir        string
	Race          bool
	Tags          string
	Toolexec      string
	Trimpath      bool
	Work          bool

	// after post-load, these are the flags to be passed to the go test command
	RenderedFlags []string `yaml:"-" json:"-" mapstructure:"-"`
	tracker       *xflagset.Decorator
	NamedFlagSet  *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

func (b *GoBuild) PostLoad() error {
	b.RenderedFlags = b.tracker.RenderFlags()
	return nil
}

func (b *GoBuild) AddFlags(flags fangs.FlagSet) {
	b.NamedFlagSet = xflagset.NewNamed()
	b.tracker = xflagset.NewDecorator(flags, b.NamedFlagSet.FlagSet("Build"))
	flags = b.tracker

	flags.StringVarP(&b.Cd, "cd", "C", "change to dir before running the command")
	flags.BoolVarP(&b.All, "all", "a", "force rebuilding of packages that are already up-to-date")
	flags.BoolVarP(&b.Race, "race", "", "enable data race detection")
	flags.BoolVarP(&b.Msan, "msan", "", "enable interoperation with memory sanitizer")
	flags.BoolVarP(&b.Asan, "asan", "", "enable interoperation with address sanitizer")
	flags.BoolVarP(&b.Work, "work", "", "print the name of the temporary work directory and do not delete it when exiting")
	flags.StringVarP(&b.Asmflags, "asmflags", "", "arguments to pass on each go tool asm invocation ([pattern=]arg list)")
	flags.StringVarP(&b.Buildmode, "buildmode", "", "build mode to use (see 'go help buildmode' for more)")
	flags.StringVarP(&b.Buildvcs, "buildvcs", "", "whether to stamp binaries with version control information (options: true, false, or auto)")
	flags.StringVarP(&b.Compiler, "compiler", "", "name of compiler to use (options: gccgo or gc)")
	flags.StringVarP(&b.Gccgoflags, "gccgoflags", "", "arguments to pass on each gccgo compiler/linker invocation ([pattern=]arg list)")
	flags.StringVarP(&b.Gcflags, "gcflags", "", "arguments to pass on each go tool compile invocation ([pattern=]arg list)")
	flags.StringVarP(&b.Installsuffix, "installsuffix", "", "a suffix to use in the name of the package installation directory")
	flags.StringVarP(&b.Ldflags, "ldflags", "", "arguments to pass on each go tool link invocation ([pattern=]arg list)")
	flags.BoolVarP(&b.Linkshared, "linkshared", "", "build code that will be linked against shared libraries previously created with -buildmode=shared")
	flags.StringVarP(&b.Mod, "mod", "", "module download mode to use (options: readonly, vendor, or mod)")
	flags.BoolVarP(&b.Modcacherw, "modcacherw", "", "leave newly-created directories in the module cache read-write instead of making them read-only")
	flags.StringVarP(&b.Modfile, "modfile", "", "read (and possibly write) an alternate go.mod file instead of the one in the module root directory")
	flags.StringVarP(&b.Overlay, "overlay", "", "read a JSON config file that provides an overlay for build operations")
	flags.StringVarP(&b.Pgo, "pgo", "", "specify the file path of a profile for profile-guided optimization (options: off, auto, <FILE>)")
	flags.StringVarP(&b.Pkgdir, "pkgdir", "", "install and load all packages from dir instead of the usual locations")
	flags.StringVarP(&b.Tags, "tags", "", "a comma-separated list of additional build tags to consider satisfied during the build (tag,list)")
	flags.BoolVarP(&b.Trimpath, "trimpath", "", "remove all file system paths from the resulting executable")
	flags.StringVarP(&b.Toolexec, "toolexec", "", "a program to use to invoke toolchain programs like vet and asm")
}
