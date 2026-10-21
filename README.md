# grampspatch

Go package for writing patch files for the GrampsPatch Gramps addon.

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/iand/grampspatch)

## About

A patch is a JSON file of change entries addressed by object handle. The user
reviews it and applies it to a Gramps tree with the GrampsPatch addon (Tools >
Family Tree Processing > Apply Patch). The addon resolves each entry against the
tree as Clean, Already applied, Conflict, Missing or Invalid, and commits the
accepted entries in one transaction that Edit > Undo reverses.

The file format is defined in `FORMAT.md` in the GrampsPatch addon source;
references below to "FORMAT §N" are to its sections. Patch files use the
extension `.gpatch.json`.

| Entry    | Content                                                                           |
| -------- | --------------------------------------------------------------------------------- |
| `add`    | A complete object dict, in the `json_data` form Gramps stores, with a new handle  |
| `update` | JSON Patch (RFC 6902) operations applied to the stored object dict                |
| `delete` | The handle, with the object's `change` value as `expect_change`                   |

The package uses only the standard library and needs no build tags. It is
designed to be used with [gogramps](https://github.com/iand/gogramps) v0.0.7 or
later, which reads the tree and supplies the stored JSON of each object.

## Status

This is an early release. Use with caution.

## Usage

A command that changes a tree:

1. Opens the Gramps database directory read-only with gogramps.
2. Reads each object to change twice: as a typed value and as its stored
   `json_data`, with `Database.GetJSON`.
3. Modifies a copy of the typed value.
4. Passes the stored JSON, the original value and the modified value to `Diff`,
   and adds the resulting operations with `Update`.
5. Builds new objects with the gogramps constructors, gives each a handle from
   `Handle`, and adds them with `Add`.
6. Writes the patch with `WriteFile`.

```Go
p := grampspatch.New(grampspatch.Schema180, "Add MyHeritage matches for T00000")

ops, err := grampspatch.Diff(stored, before, after)
if err != nil {
	return err
}
if len(ops) > 0 {
	if err := p.Update(grampspatch.ClassPerson, before.Handle, ops); err != nil {
		return err
	}
}

handle, err := grampspatch.Handle("DNAM", testID, matchGUID)
if err != nil {
	return err
}
// ... build a gogramps.DNAMatch with that handle ...
if err := p.Add(grampspatch.ClassDNAMatch, handle, match); err != nil {
	return err
}

return p.WriteFile("myheritage-matches-20261021.gpatch.json")
```

A patch built this way applies once: opened a second time, every entry resolves
to Already applied.

## API

### Patch

| Method      | Behaviour                                                                                                     |
| ----------- | ------------------------------------------------------------------------------------------------------------- |
| `New`       | Sets `Schema` and `Message`; `Created` is set to the current time.                                            |
| `Add`       | Marshals `obj`; returns an error when its `_class` differs from `class` or its `handle` differs from `handle`. |
| `Update`    | Returns an error when `ops` is empty, or when an operation addresses a forbidden path (FORMAT §5).            |
| `Delete`    | Always writes `expect_change`.                                                                                |
| all three   | Return an error for an empty class or handle, and for a handle already used in the patch.                     |
| `Len`       | Returns the number of change entries.                                                                         |
| `Encode`    | Writes the envelope then the changes in the order added, indented by two spaces, with a final newline.        |
| `WriteFile` | Encodes to a temporary file in the target directory and renames it into place.                               |

`Update` does not write `expect_change`. The `test` operations guard the
members that change, and `expect_change` would make any unrelated edit to the
object a Conflict.

`Schema172` (Gramps 6.1) and `Schema180` (the dna-core development branch) name
the schema versions for the envelope. `ClassPerson` through `ClassDNAMatch` name
the primary object classes used in change entries.

### Op constructors

| Function   | Result                                                                     |
| ---------- | -------------------------------------------------------------------------- |
| `Test`     | `{"op":"test","path":…,"value":…}`                                         |
| `Replace`  | `{"op":"replace","path":…,"value":…}`                                      |
| `AddValue` | `{"op":"add","path":…,"value":…}`; named to avoid a clash with `Patch.Add` |
| `Remove`   | `{"op":"remove","path":…}`                                                 |
| `Pointer`  | Joins segments into a JSON Pointer, escaping `~` as `~0` and `/` as `~1`   |

A `null` value is encoded as `null`, not omitted.

### Diff

`Diff` returns the operations that turn the stored object into `after`, in the
form FORMAT §8 describes for patches that apply once.

1. Marshal `before` and compare it with `stored`. A difference returns an error
   naming the first differing path. This detects a type that does not
   round-trip, which would otherwise delete or alter data the command did not
   touch.
2. Marshal `after`. A top-level member present in only one of `stored` and
   `after` returns an error.
3. For each top-level member, in the order of `stored`, whose value differs:
   emit `test` of the stored value, then `replace` with the new value.
4. A difference in `handle`, `change` or `_class` returns an error.
5. No differences return an empty slice; the caller then writes no change.

Values are compared as FORMAT §5 defines: objects by member regardless of order,
arrays element by element, numbers by value (`1` equals `1.0`), and booleans
distinct from numbers (`false` differs from `0`).

Diffing whole top-level members makes a concurrent edit to the same member a
Conflict, and makes a second application Already applied.

### Handle

`Handle` returns `prefix` followed by the first 16 characters of the unpadded
base-32 encoding of the SHA-256 hash of the tokens, each followed by a zero
byte. The prefix must be one to eight upper-case letters or digits. The same
prefix and tokens always give the same handle, so a second run adds objects that
GrampsPatch resolves as Already applied.

## Tests

`testdata/data-dna.gpatch.json` is the example patch from the GrampsPatch
addon. `TestFixture` rebuilds it through the package and compares the result,
apart from `created`.

## License

This is free and unencumbered software released into the public domain. For more
information, see <http://unlicense.org/> or the accompanying [`UNLICENSE`](UNLICENSE) file.
