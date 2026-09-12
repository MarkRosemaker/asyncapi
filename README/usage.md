```go
package main

import (
    "fmt"

    "github.com/MarkRosemaker/asyncapi"
)

func main() {
    doc, err := asyncapi.LoadFromFile("path/to/asyncapi.json") // or asyncapi.yaml
    if err != nil {
        fmt.Println("Error parsing spec:", err)
        return
    }

    if err := doc.Validate(); err != nil {
        fmt.Println("Error validating spec:", err)
        return
    }

    // sort the keys of the servers, channels, operations and components in alphabetical order
    doc.SortMaps()

    // write an improved version of your spec, as JSON or as YAML
    if err := doc.WriteToFile("path/to/asyncapi.json"); err != nil {
        fmt.Println("Error writing to file:", err)
        return
    }
}
```
