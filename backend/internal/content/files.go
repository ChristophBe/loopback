package content

import (
	"io/fs"
	"path"
	"slices"
	"strings"
	"unicode/utf8"
)

// readDir reads the files of a lesson directory named by the starterFiles or
// testFiles field. dir is relative to the directory of the lesson file. It
// reports a missing or unreadable directory, and every file that is not listed
// in owned, and returns the contents by path relative to dir.
func (f *file) readDir(fsys fs.FS, field, dir string, owned []string) map[string]string {
	root := path.Join(path.Dir(f.name), dir)
	if info, err := fs.Stat(fsys, root); err != nil || !info.IsDir() {
		f.addf(field, "directory %q does not exist", dir)
		return nil
	}
	files := map[string]string{}
	err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(p, root+"/")
		switch data, err := fs.ReadFile(fsys, p); {
		case !d.Type().IsRegular():
			f.addf(field, "%q in %q is not a regular file", rel, dir)
		case !slices.Contains(owned, rel):
			f.addf(field, "file %q in %q is not listed in ownedFiles", rel, dir)
		case err != nil:
			f.addf(field, "cannot read %q in %q: %v", rel, dir, err)
		case !utf8.Valid(data) || strings.ContainsRune(string(data), 0):
			f.addf(field, "file %q in %q is not UTF-8 text", rel, dir)
		default:
			files[rel] = string(data)
		}
		return nil
	})
	if err != nil {
		f.addf(field, "cannot read directory %q: %v", dir, err)
	}
	return files
}

// lessonFiles reads the starter and test directories of a lesson. When at
// least one is declared, every entry of owned must be in one of them.
func (f *file) lessonFiles(fsys fs.FS, starter, tests *string, owned []string) (starterFiles, testFiles map[string]string) {
	if starter == nil && tests == nil {
		return nil, nil
	}
	if starter != nil {
		starterFiles = f.readDir(fsys, "starterFiles", *starter, owned)
	}
	if tests != nil {
		testFiles = f.readDir(fsys, "testFiles", *tests, owned)
	}
	for _, o := range owned {
		_, inStarter := starterFiles[o]
		_, inTests := testFiles[o]
		if !inStarter && !inTests {
			f.addf("ownedFiles", "file %q is missing from the %s", o, declared(starter, tests))
		}
	}
	return starterFiles, testFiles
}

func declared(starter, tests *string) string {
	switch {
	case starter != nil && tests != nil:
		return "starterFiles and testFiles directories"
	case starter != nil:
		return "starterFiles directory"
	}
	return "testFiles directory"
}
