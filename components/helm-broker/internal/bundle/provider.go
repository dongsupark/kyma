package bundle

import (
	"io/ioutil"

	"github.com/ghodss/yaml"
	"github.com/kyma-project/kyma/components/helm-broker/internal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

// CompleteBundleProvider provides CompleteBundles from a repository.
type CompleteBundleProvider struct {
	log          *logrus.Entry
	bundleLoader bundleLoader
	repo         repository
}

// CompleteBundle aggregates a bundle with his chart(s)
type CompleteBundle struct {
	Bundle *internal.Bundle
	Charts []*chart.Chart
}

// ID returns the ID of the bundle
func (b *CompleteBundle) ID() internal.BundleID {
	return b.Bundle.ID
}

// NewProvider returns new instance of CompleteBundleProvider.
func NewProvider(repo repository, bundleLoader bundleLoader, log logrus.FieldLogger) *CompleteBundleProvider {
	return &CompleteBundleProvider{
		repo:         repo,
		bundleLoader: bundleLoader,
		log:          log.WithField("service", "bundle-CompleteBundleProvider"),
	}
}

// ProvideBundles returns a list of bundles with his charts as CompleteBundle instances.
// In case of bundle processing errors, the won't be stopped - next bundle is processed.
func (l *CompleteBundleProvider) ProvideBundles() ([]CompleteBundle, error) {
	idx, err := l.getIndex()
	if err != nil {
		return nil, err
	}

	var allBundles []*internal.Bundle
	var allCharts []*chart.Chart
	var items []CompleteBundle
	for entryName, versions := range idx.Entries {
		for _, v := range versions {
			bundle, charts, err := l.loadBundlesAndCharts(entryName, v.Version)
			if err != nil {
				l.log.Warnf("Could not load bundle: %s", err.Error())
				continue
			}
			allBundles = append(allBundles, bundle)
			allCharts = append(allCharts, charts...)
			items = append(items, CompleteBundle{
				Bundle: bundle,
				Charts: charts,
			})
		}
	}
	l.log.Debug("Loading bundles completed.")
	return items, nil
}

func (l *CompleteBundleProvider) getIndex() (*indexDTO, error) {
	idxReader, idxCloser, err := l.repo.IndexReader()
	if err != nil {
		return nil, errors.Wrap(err, "while getting index file")
	}
	defer idxCloser()

	bytes, err := ioutil.ReadAll(idxReader)
	if err != nil {
		return nil, errors.Wrap(err, "while reading all index file")
	}
	idx := indexDTO{}
	if err = yaml.Unmarshal(bytes, &idx); err != nil {
		return nil, errors.Wrap(err, "while unmarshaling index file")
	}
	return &idx, nil
}

func (l *CompleteBundleProvider) loadBundlesAndCharts(entryName Name, version Version) (*internal.Bundle, []*chart.Chart, error) {
	bundleReader, bundleCloser, err := l.repo.BundleReader(entryName, version)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "while reading bundle archive for name [%s] and version [%v]", entryName, version)
	}
	defer bundleCloser()

	bundle, charts, err := l.bundleLoader.Load(bundleReader)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "while loading bundle and charts for bundle [%s] and version [%s]", entryName, version)
	}

	return bundle, charts, nil
}
