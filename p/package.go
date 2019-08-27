package p

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
)


type Package struct {
	Name        string  `yaml:"name" json:"name"`
	Title       *string `yaml:"title,omitempty" json:"title,omitempty"`
	Version     string  `yaml:"version" json:"version"`
	Description string  `yaml:"description" json:"description"`
}

type Manifest struct {
	Package     `yaml:",inline" json:",inline"`
	Requirement struct {
		Kibana struct {
			Min string `yaml:"version.min" json:"version.min"`
			Max string `yaml:"version.max" json:"version.max"`
		} `yaml:"kibana" json:"kibana"`
	} `yaml:"requirement" json:"requirement"`
	Screenshots []Image `yaml:"screenshots,omitempty" json:"screenshots,omitempty"`
	Icons       []Image `yaml:"icons,omitempty" json:"icons,omitempty"`
}

type Image struct {
	Src   string `yaml:"src" json:"src,omitempty"`
	Title string `yaml:"title" json:"title,omitempty"`
	Size  string `yaml:"size" json:"size,omitempty"`
	Type  string `yaml:"type" json:"type,omitempty"`
}

func (i Image) getPath(m *Manifest) string {
	return "/package/" + m.Name + "-" + m.Version + i.Src
}

func ReadManifest(packagesPath, p string) (*Manifest, error) {

	manifest, err := ioutil.ReadFile(packagesPath + "/" + p + "/manifest.yml")
	if err != nil {
		return nil, err
	}

	var m = &Manifest{}
	err = yaml.Unmarshal(manifest, m)
	if err != nil {
		return nil, err
	}

	if m.Icons != nil {
		for k, i := range m.Icons {
			m.Icons[k].Src = i.getPath(m)
		}
	}

	if m.Screenshots != nil {
		for k, s := range m.Screenshots {
			m.Screenshots[k].Src = s.getPath(m)
		}
	}

	return m, nil
}

