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
// flags to be passed to the `go test` or `go build` command. These options control how Go compiles test binaries.
type GoBuild struct {
	// All forces rebuilding of packages that are already up-to-date.
	All bool
	// Asan enables interoperation with address sanitizer.
	Asan bool
	// Asmflags are arguments to pass on each go tool asm invocation.
	Asmflags string
	// Buildmode sets the build mode to use (see 'go help buildmode').
	Buildmode string
	// Buildvcs controls whether to stamp binaries with version control information.
	Buildvcs string
	// Cd changes to this directory before running the command.
	Cd string
	// Compiler specifies the compiler to use (gccgo or gc).
	Compiler string
	// Gccgoflags are arguments to pass to the gccgo compiler/linker.
	Gccgoflags string
	// Gcflags are arguments to pass on each go tool compile invocation.
	Gcflags string
	// Installsuffix is a suffix to use in the package installation directory name.
	Installsuffix string
	// Ldflags are arguments to pass on each go tool link invocation.
	Ldflags string
	// Linkshared builds code that will be linked against shared libraries.
	Linkshared bool
	// Mod sets the module download mode (readonly, vendor, or mod).
	Mod string
	// Modcacherw leaves newly-created module cache directories read-write.
	Modcacherw bool
	// Modfile reads an alternate go.mod file.
	Modfile string
	// Msan enables interoperation with memory sanitizer.
	Msan bool
	// Overlay reads a JSON config file providing an overlay for build operations.
	Overlay string
	// Pgo specifies the file path of a profile for profile-guided optimization.
	Pgo string
	// Pkgdir installs and loads all packages from this directory.
	Pkgdir string
	// Race enables data race detection.
	Race bool
	// Tags is a comma-separated list of build tags to consider satisfied.
	Tags string
	// Toolexec is a program to use to invoke toolchain programs.
	Toolexec string
	// Trimpath removes all file system paths from the resulting executable.
	Trimpath bool
	// Work prints the temporary work directory name and doesn't delete it.
	Work bool

	// RenderedFlags contains the final flags to be passed to the go test command after post-load processing.
	RenderedFlags []string `yaml:"-" json:"-" mapstructure:"-"`
	tracker       *xflagset.Decorator
	NamedFlagSet  *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

// DefaultGoBuild returns build options with Go's default build configuration.
func DefaultGoBuild() GoBuild {
	return GoBuild{}
}

// PostLoad renders all build flags into a format suitable for passing to `go test`.
func (b *GoBuild) PostLoad() error {
	b.RenderedFlags = b.tracker.RenderFlags()
	return nil
}

// AddFlags registers all go build-related flags with the flag set.
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
