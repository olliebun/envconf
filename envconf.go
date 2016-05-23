/*
Package envconf helps to configure Go applications from the process
environment, rather than needing to deploy and parse config files. It mimics
the API of other popular config packages such as gcfg.

envconf provides a simple and powerful mechanism to parse configuration: it
takes a struct and a function of the signature func(string) string and
populates the struct. This give the user the flexibility of specifying where
the configuration comes from, but the power of type-safe config parsing.

This way administrators don't need to juggle config files around, and large
projects don't have to worry about subsystems trying to find their config in
the global process state.

envconf allows the package user to define a type matching the config
variables they want to pull out of the environment.

Usage

Define a struct literal or an instance of a struct type and call ReadConfigEnv:

	var serverConfig struct {
		Port int    `required:"true"`
		Bind string `default:"0.0.0.0"`
	}
	err := envconf.ReadConfigEnv(&serverConfig)
	// Deal with error here

This will look up both PORT and BIND in the process environment and populate
them in the config struct - provided that the value found for Port can be
parsed as an int.

You can also set a prefix:

	err := envconf.ReadConfigEnvPrefix("MYSERVER_", &serverConfig)

This will behave in the same way as above, but will look for the environment
variables MYSERVER_PORT and MYSERVER_BIND. This provides a simple way to
namespace the environment variables.

Types

Three basic types are supported: int, bool and string. Slices of these types
are also supported; this struct is valid:

	type AlarmConfig {
		DaysOfWeek []int
		Addresses  []string
		Active     bool
	}

envconf expects comma-separated values for slice types.

Tags

As seen above, envconf understands the "required" and "default" tags. These do
what they sound like.


*/
package envconf

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// ReadConfig reads from this getter func into a struct.
//
// Must be passed a struct or a pointer to a struct.
func ReadConfig(conf interface{}, getter func(string) string) error {
	var (
		v       = reflect.ValueOf(conf)
		missing []string
		err     error
	)

	if v.Type().Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Type().Kind() != reflect.Struct {
		return fmt.Errorf(
			"Invalid kind for config: %v", v.Type().Kind())
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		fieldVal := v.Field(i)
		kind := field.Type.Kind()

		input := getter(strings.ToUpper(field.Name))

		if len(field.PkgPath) > 0 {
			// ignore unexported
			continue
		} else if len(input) == 0 && field.Tag.Get("required") == "true" {
			missing = append(missing, strings.ToUpper(field.Name))
			continue
		} else if defaul := field.Tag.Get("default"); len(input) == 0 && len(defaul) > 0 {
			input = defaul
		} else if len(input) == 0 {
			continue
		}

		switch kind {
		default:
			return fmt.Errorf(
				"Invalid kind for config field %s: %v", field.Name, kind)
		case reflect.String:
			fieldVal.Set(reflect.ValueOf(input))
		case reflect.Int:
			if i, err := strconv.Atoi(input); err != nil {
				return err
			} else {
				fieldVal.Set(reflect.ValueOf(i))
			}
		case reflect.Bool:
			if b, err := strconv.ParseBool(input); err != nil {
				return err
			} else {
				fieldVal.SetBool(b)
			}
		case reflect.Slice:
			// Complex case
			spl := strings.Split(input, ",")
			switch field.Type {
			default:
				return fmt.Errorf(
					"Invalid kind for config field %s: %v", field.Name, field.Type)
			case reflect.SliceOf(reflect.TypeOf("")):
				sl := make([]string, len(spl))
				for i, iv := range spl {
					sl[i] = iv
				}
				fieldVal.Set(reflect.ValueOf(sl))
			case reflect.SliceOf(reflect.TypeOf(1)):
				sl := make([]int, len(spl))
				for i, iv := range spl {
					if intval, err := strconv.Atoi(iv); err != nil {
						return err
					} else {
						sl[i] = intval
					}
				}
				fieldVal.Set(reflect.ValueOf(sl))
			case reflect.SliceOf(reflect.TypeOf(true)):
				sl := make([]bool, len(spl))
				for i, iv := range spl {
					if bval, err := strconv.ParseBool(iv); err != nil {
						return err
					} else {
						sl[i] = bval
					}

				}
				fieldVal.Set(reflect.ValueOf(sl))
			}
		}

	}

	if len(missing) > 0 {
		err = fmt.Errorf(
			"Missing config fields: %s", strings.Join(missing, ", "))
	}

	return err
}

// ReadConfigEnv reads config from the process environment. A shortcut for:
//	envconf.ReadConfig(conf, os.GetEnv)
func ReadConfigEnv(conf interface{}) error {
	return ReadConfig(conf, os.Getenv)
}

// a map wrapper for testing
type mapgetter map[string]string

func (t mapgetter) get(s string) string { return t[s] }

// ReadConfigMap reads config from this map.
func ReadConfigMap(conf interface{}, m map[string]string) error {
	return ReadConfig(conf, mapgetter(m).get)
}

// ReadConfigenvPrefix reads config from the environment with a set prefix on
// every environment variable.
func ReadConfigEnvPrefix(prefix string, conf interface{}) error {
	getter := func(k string) string {
		return os.Getenv(fmt.Sprintf("%s%s", prefix, k))
	}
	return ReadConfig(conf, getter)
}
