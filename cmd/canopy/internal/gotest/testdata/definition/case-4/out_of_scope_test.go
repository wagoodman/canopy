package case_4

// the important part about this test is that this literal is out of scope when looking at the AST
var expectedZipArchiveEntries = []string{
	"zip-source/",
	"zip-source/some-dir/",
	"zip-source/some-dir/a-file.txt",
}
