# Custom Data Management

The `Custom Data` feature implements a storage for 'any' data, which can be used by other components for data persistence.

Because the schema of the data being persisted varies from components, a `CustomDataKind` must be defined to distinguish the data.

## CustomDataKind

The YAML example below defines a CustomDataKind:

```yaml
name: kind1
idField: name
jsonSchema:
  type: object
  properties:
    name:
      type: string
      required: true
```

The `name` field is required, it is the kind name of custom data items of this kind.
The `idField` is optional, and its default value is `name`, the value of this field of a data item is used as its identifier.
The `jsonSchema` is optional, if provided, data items of this kind will be validated against this [JSON Schema](http://json-schema.org/).

## CustomData

CustomData is a map, the keys of this map must be strings while the values can be any valid JSON values, but the keys of a nested map must be strings too.

A CustomData item must contain the `idField` defined by its corresponding CustomDataKind, for example, the data items of the kind defined in the above example must contain the `name` field as their identifiers.

Below is an example of a CustomData:

```yaml
name: data1
field1: 12
field2: abc
field3: [1, 2, 3, 4]
```

## API

* **Create a CustomDataKind**
        * **URL**: http://{ip}:{port}/apis/v2/customdatakinds
        * **Method**: POST
        * **Body**: CustomDataKind definition is YAML.

* **Update a CustomDataKind**
        * **URL**: http://{ip}:{port}/apis/v2/customdatakinds
        * **Method**: PUT
        * **Body**: CustomDataKind definition is YAML.

* **Query the definition of a CustomDataKind**
        * **URL**: http://{ip}:{port}/apis/v2/customdatakinds/{kind name}
        * **Method**: GET

* **List the definition of all CustomDataKind**
        * **URL**: http://{ip}:{port}/apis/v2/customdatakinds
        * **Method**: GET

* **Delete a CustomDataKind**
        * **URL**: http://{ip}:{port}/apis/v2/customdatakinds/{kind name}
        * **Method**: DELETE

* **Create a CustomData**
        * **URL**: http://{ip}:{port}/apis/v2/customdata/{kind name}
        * **Method**: POST
        * **Body**: CustomData definition is YAML.

* **Update a CustomData**
        * **URL**: http://{ip}:{port}/apis/v2/customdata/{kind name}
        * **Method**: PUT
        * **Body**: CustomData definition is YAML.

* **Query the definition of a CustomData**
        * **URL**: http://{ip}:{port}/apis/v2/customdata/{kind name}/{data id}
        * **Method**: GET

* **List the definition of all CustomData of a kind**
        * **URL**: http://{ip}:{port}/apis/v2/customdata/{kind name}
        * **Method**: GET

* **Delete a CustomData**
        * **URL**: http://{ip}:{port}/apis/v2/customdata/{kind name}/{data id}
        * **Method**: DELETE

* **Delete all CustomData of a kind**
        * **URL**: http://{ip}:{port}/apis/v2/customdata/{kind name}
        * **Method**: DELETE

* **Bulk update**
        * **URL**: http://{ip}:{port}/apis/v2/customdata/{kind name}/items
        * **Method**: POST
        * **Body**: A change request in YAML, as defined below.

```yaml
rebuild: false
delete: [data1, data2]
list:
- name: data3
  field1: 12
- name: data4
  field1: foo
```
When `rebuild` is true (default is false), all existing data items are deleted before processing the data items in `list`. `delete` is an array of data identifiers to be deleted, this array is ignored when `rebuild` is true. `list` is an array of data items to be created or updated.
