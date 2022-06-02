package common

import (
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/anchore/syft/syft/artifact"
	"github.com/anchore/syft/syft/pkg"
	"github.com/anchore/syft/syft/source"
)

func parser(_ string, reader io.Reader) ([]*pkg.Package, []artifact.Relationship, error) {
	contents, err := ioutil.ReadAll(reader)
	if err != nil {
		panic(err)
	}
	return []*pkg.Package{
		{
			Name: string(contents),
		},
	}, nil, nil
}

func TestGenericCataloger(t *testing.T) {
	globParsers := map[string]ParserFn{
		"**/*.txt": parser, // this glob should capture all files, including the empty one
	}
	pathParsers := map[string]ParserFn{
		"test-fixtures/another-path.txt": parser,
		"test-fixtures/last/path.txt":    parser,
	}
	upstream := "some-other-cataloger"

	expectedSelection := []string{"test-fixtures/last/path.txt", "test-fixtures/another-path.txt", "test-fixtures/a-path.txt", "test-fixtures/empty.txt"}
	resolver := source.NewMockResolverForPaths(expectedSelection...)
	cataloger := NewGenericCataloger(pathParsers, globParsers, upstream)

	expectedPkgs := make(map[string]pkg.Package)
	for _, path := range expectedSelection {
		expectedPkgs[path] = pkg.Package{
			FoundBy: upstream,
			Name:    fmt.Sprintf("%s file contents!", path),
		}
	}

	actualPkgs, _, err := cataloger.Catalog(resolver)
	assert.NoError(t, err)
	// empty.txt won't become a package
	assert.Len(t, actualPkgs, len(expectedPkgs)-1)

	for _, p := range actualPkgs {
		ref := p.Locations.ToSlice()[0]
		exP, ok := expectedPkgs[ref.RealPath]
		if !ok {
			t.Errorf("missing expected pkg: ref=%+v", ref)
			continue
		}

		if p.FoundBy != exP.FoundBy {
			t.Errorf("bad upstream: %s", p.FoundBy)
		}

		if exP.Name != p.Name {
			t.Errorf("bad contents mapping: %+v", p.Locations)
		}
	}
}

func Test_removeRelationshipsWithArtifactIDs(t *testing.T) {
	one := &pkg.Package{Name: "one", Version: "1.0"}
	two := &pkg.Package{Name: "two", Version: "1.0"}
	three := &pkg.Package{Name: "three", Version: "1.0"}
	four := &pkg.Package{Name: "four", Version: "bla"}
	five := &pkg.Package{Name: "five", Version: "1.0"}

	pkgs := make([]artifact.Identifiable, 0)
	for _, p := range []*pkg.Package{one, two, three, four, five} {
		// IDs are necessary for comparison
		p.SetID()
		pkgs = append(pkgs, p)
	}

	type args struct {
		remove        map[artifact.ID]artifact.Identifiable
		relationships []artifact.Relationship
	}
	tests := []struct {
		name string
		args args
		want []artifact.Relationship
	}{
		{
			name: "nothing-to-remove",
			args: args{
				relationships: []artifact.Relationship{
					{From: one, To: two},
				},
			},
			want: []artifact.Relationship{
				{From: one, To: two},
			},
		},
		{
			name: "remove-all-relationships",
			args: args{
				remove: map[artifact.ID]artifact.Identifiable{
					one.ID():   one,
					three.ID(): three,
				},
				relationships: []artifact.Relationship{
					{From: one, To: two},
					{From: two, To: three},
					{From: three, To: four},
				},
			},
			want: []artifact.Relationship(nil),
		},
		{
			name: "remove-half-of-relationships",
			args: args{
				remove: map[artifact.ID]artifact.Identifiable{
					one.ID(): one,
				},
				relationships: []artifact.Relationship{
					{From: one, To: two},
					{From: one, To: three},
					{From: two, To: three},
					{From: three, To: four},
				},
			},
			want: []artifact.Relationship{
				{From: two, To: three},
				{From: three, To: four},
			},
		},
		{
			name: "remove-repeated-relationships",
			args: args{
				remove: map[artifact.ID]artifact.Identifiable{
					one.ID(): one,
					two.ID(): two,
				},
				relationships: []artifact.Relationship{
					{From: one, To: two},
					{From: one, To: three},
					{From: two, To: three},
					{From: two, To: three},
					{From: three, To: four},
					{From: four, To: five},
				},
			},
			want: []artifact.Relationship{
				{From: three, To: four},
				{From: four, To: five},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, removeRelationshipsWithArtifactIDs(tt.args.remove, tt.args.relationships), "removeRelationshipsWithArtifactIDs(%v, %v)", tt.args.remove, tt.args.relationships)
		})
	}
}
