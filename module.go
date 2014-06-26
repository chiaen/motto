package motto

import (
    "github.com/robertkrimen/otto"
    "io/ioutil"
    "path/filepath"
    "errors"
)

type ModuleLoader func(*Motto) (otto.Value, error)

func CreateLoaderFromSource(source, pwd string) ModuleLoader {
    return func (vm *Motto) (otto.Value, error) {
        source = "(function(module) {var require = module.require;var exports = module.exports;\n" + source + "\n})"

        // Provide the "require" method in the module scope.
        jsRequire := func(call otto.FunctionCall) otto.Value {
            jsModuleName := call.Argument(0).String()

            moduleValue, err := vm.Require(jsModuleName, pwd)
            if err != nil {
                jsException(vm, "Error", "motto: " + err.Error())
            }

            return moduleValue
        }

        jsModule, _ := vm.Object(`({exports: {}})`)
        jsModule.Set("require", jsRequire)
        jsExports, _ := jsModule.Get("exports")

        // Run the module source, with "jsModule" as the "module" varaible, "jsExports" as "this"(Nodejs capable).
        moduleReturn, err := vm.Call(source, jsExports, jsModule)
        if err != nil {
            return otto.UndefinedValue(), err
        }

        var moduleValue otto.Value
        if !moduleReturn.IsUndefined() {
            moduleValue = moduleReturn
            jsModule.Set("exports", moduleValue)
        } else {
            moduleValue, _ = jsModule.Get("exports")
        }

        return moduleValue, nil
    }
}

func CreateLoaderFromFile(filename string) ModuleLoader {
    return func (vm *Motto) (otto.Value, error) {
        source, err := ioutil.ReadFile(filename)

        if err != nil {
            return otto.UndefinedValue(), err
        }

        // load json
        if filepath.Ext(filename) == ".json" {
            return vm.Call("JSON.parse", nil, string(source))
        }

        pwd := filepath.Dir(filename)

        return CreateLoaderFromSource(string(source), pwd)(vm)
    }
}

// Find a file module by name
func FindFileModule(name, pwd string, paths []string) (string, error) {
    if len(name) == 0 {
        return "", errors.New("Empty module name")
    }

    var choices []string
    if name[0] == '.' || name[0] == '/' {
        if name[0] == '.' {
            name = filepath.Join(pwd, name)
        }

        choices = append(choices, name)
        ext := filepath.Ext(name)
        if ext != ".js" && ext != ".json" {
            choices = append(choices, name + ".js", name + ".json")
        }
    } else {
        if pwd != "" {
            choices = append(choices, filepath.Join(pwd, "node_modules", name))
        }

        for _, v := range paths {
            choices = append(choices, filepath.Join(v), name)
        }
    }

    for _, v := range choices {
        ok, err := isDir(v)
        if err != nil {
            return "", err
        }

        if ok {
            packageJsonFilename := filepath.Join(v, "package.json")
            ok, err := isFile(packageJsonFilename)
            if err != nil {
                return "", err
            }

            var entryPoint string
            if ok {
                entryPoint, err = parsePackageEntryPoint(packageJsonFilename)
                if err != nil {
                    return "", err
                }
            } else {
                entryPoint = "./index.js"
            }

            return filepath.Abs(filepath.Join(v, entryPoint))
        }

        ok, err = isFile(v)
        if err != nil {
            return "", err
        }

        if ok {
            return filepath.Abs(v)
        }
    }

    return "", errors.New("Module not found: " + name)
}