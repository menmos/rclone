package configfile

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var configData = `[one]
type = number1
fruit = potato

[two]
type = number2
fruit = apple
topping = nuts

[three]
type = number3
fruit = banana

`

// Fill up a temporary config file with the testdata filename passed in
func setConfigFile(t *testing.T, data string) func() {
	out, err := ioutil.TempFile("", "rclone-configfile-test")
	require.NoError(t, err)
	filePath := out.Name()

	_, err = out.Write([]byte(data))
	require.NoError(t, err)

	require.NoError(t, out.Close())

	old := config.ConfigPath
	config.ConfigPath = filePath
	return func() {
		config.ConfigPath = old
		_ = os.Remove(filePath)
	}
}

func TestConfigFile(t *testing.T) {
	defer setConfigFile(t, configData)()
	data := &Storage{}

	require.NoError(t, data.Load())

	t.Run("Read", func(t *testing.T) {
		t.Run("Serialize", func(t *testing.T) {
			buf, err := data.Serialize()
			require.NoError(t, err)
			assert.Equal(t, configData, buf)
		})
		t.Run("HasSection", func(t *testing.T) {
			assert.True(t, data.HasSection("one"))
			assert.False(t, data.HasSection("missing"))
		})
		t.Run("GetSectionList", func(t *testing.T) {
			assert.Equal(t, []string{
				"one",
				"two",
				"three",
			}, data.GetSectionList())
		})
		t.Run("GetKeyList", func(t *testing.T) {
			assert.Equal(t, []string{
				"type",
				"fruit",
				"topping",
			}, data.GetKeyList("two"))
			assert.Equal(t, []string(nil), data.GetKeyList("unicorn"))
		})
		t.Run("GetValue", func(t *testing.T) {
			value, ok := data.GetValue("one", "type")
			assert.True(t, ok)
			assert.Equal(t, "number1", value)
			value, ok = data.GetValue("three", "fruit")
			assert.True(t, ok)
			assert.Equal(t, "banana", value)
			value, ok = data.GetValue("one", "typeX")
			assert.False(t, ok)
			assert.Equal(t, "", value)
			value, ok = data.GetValue("threeX", "fruit")
			assert.False(t, ok)
			assert.Equal(t, "", value)
		})
	})

	//defer setConfigFile(configData)()

	t.Run("Write", func(t *testing.T) {
		t.Run("SetValue", func(t *testing.T) {
			data.SetValue("one", "extra", "42")
			data.SetValue("two", "fruit", "acorn")

			buf, err := data.Serialize()
			require.NoError(t, err)
			assert.Equal(t, `[one]
type = number1
fruit = potato
extra = 42

[two]
type = number2
fruit = acorn
topping = nuts

[three]
type = number3
fruit = banana

`, buf)
			t.Run("DeleteKey", func(t *testing.T) {
				data.DeleteKey("one", "type")
				data.DeleteKey("two", "missing")
				data.DeleteKey("three", "fruit")
				buf, err := data.Serialize()
				require.NoError(t, err)
				assert.Equal(t, `[one]
fruit = potato
extra = 42

[two]
type = number2
fruit = acorn
topping = nuts

[three]
type = number3

`, buf)
				t.Run("DeleteSection", func(t *testing.T) {
					data.DeleteSection("two")
					data.DeleteSection("missing")
					buf, err := data.Serialize()
					require.NoError(t, err)
					assert.Equal(t, `[one]
fruit = potato
extra = 42

[three]
type = number3

`, buf)
					t.Run("Save", func(t *testing.T) {
						require.NoError(t, data.Save())
						buf, err := ioutil.ReadFile(config.ConfigPath)
						require.NoError(t, err)
						assert.Equal(t, `[one]
fruit = potato
extra = 42

[three]
type = number3

`, string(buf))
					})
				})
			})
		})
	})
}

func TestConfigFileReload(t *testing.T) {
	defer setConfigFile(t, configData)()
	data := &Storage{}

	require.NoError(t, data.Load())

	value, ok := data.GetValue("three", "appended")
	assert.False(t, ok)
	assert.Equal(t, "", value)

	// Now write a new value on the end
	out, err := os.OpenFile(config.ConfigPath, os.O_APPEND|os.O_WRONLY, 0777)
	require.NoError(t, err)
	fmt.Fprintln(out, "appended = what magic")
	require.NoError(t, out.Close())

	// And check we magically reloaded it
	value, ok = data.GetValue("three", "appended")
	assert.True(t, ok)
	assert.Equal(t, "what magic", value)
}

func TestConfigFileDoesNotExist(t *testing.T) {
	defer setConfigFile(t, configData)()
	data := &Storage{}

	require.NoError(t, os.Remove(config.ConfigPath))

	err := data.Load()
	require.Equal(t, config.ErrorConfigFileNotFound, err)

	// check that using data doesn't crash
	value, ok := data.GetValue("three", "appended")
	assert.False(t, ok)
	assert.Equal(t, "", value)
}
