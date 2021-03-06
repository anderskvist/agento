package muninpluginrunner

import (
	"bufio"
	"regexp"
	"strconv"

	"github.com/abrander/agento/plugins"
	"github.com/abrander/agento/timeseries"
)

func init() {
	plugins.Register("muninpluginrunner", newMuninPluginRunner)
}

// MuninPluginRunner will retrieve stub status.
type MuninPluginRunner struct {
	Command   string `toml:"command" json:"command" description:"Command to run"`
	Arguments string `toml:"arguments" json:"arguments" description:"Arguments to command"`
	Prefix    string `toml:"prefix" json:"prefix" description:"Prefix to output variables"`

	kv []keyValue
}

type keyValue struct {
	key   string
	value float64
}

func newMuninPluginRunner() interface{} {
	return new(MuninPluginRunner)
}

// Gather will execute command (with arguments) and read each line in output.
// Gather expect output to be munin plugin style:
// http://munin-monitoring.org/wiki/HowToWritePlugins
func (m *MuninPluginRunner) Gather(transport plugins.Transport) error {
	stdout, _, err := transport.Exec(m.Command, m.Arguments)

	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		re := regexp.MustCompile("^(.*).value ([0-9]+(\\.([0-9])*)?)$")
		matches := re.FindAllStringSubmatch(scanner.Text(), -1)

		if len(matches) == 1 {
			value, err := strconv.ParseFloat(matches[0][2], 64)

			if err != nil {
				return err
			}

			kv := keyValue{}
			kv.key = matches[0][1]
			kv.value = value

			m.kv = append(m.kv, kv)
		}
	}

	return nil
}

// GetPoints will return one point per line (keys) in output from command.
func (m *MuninPluginRunner) GetPoints() []*timeseries.Point {
	points := make([]*timeseries.Point, len(m.kv))

	for i, kv := range m.kv {

		var key string
		if m.Prefix != "" {
			key = m.Prefix + "." + kv.key
		} else {
			key = kv.key
		}

		points[i] = plugins.SimplePoint(key, kv.value)
	}
	return points
}

func (m *MuninPluginRunner) GetDoc() *plugins.Doc {
	doc := plugins.NewDoc("Munin-plugin-runner doesn't have any measurements, but will read munin plugin format and use key and values.")

	return doc
}

// Ensure compliance.
var _ plugins.Agent = (*MuninPluginRunner)(nil)
