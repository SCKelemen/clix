package format

// FiltersTopic is a help topic about the list flags that ListFlags adds,
// like gcloud's "topic filters", for an app to add:
//
//	app.AddHelpTopic("filters", "Filter, sort and limit lists", format.FiltersTopic)
const FiltersTopic = `Commands that list resources take three flags, applied in this order:

  --filter EXPRESSION   keep the items the expression selects
  --sort-by FIELD,...   sort them; ~FIELD sorts by FIELD in descending order
  --limit N             keep at most N of them ("unlimited" keeps all)

The output flags, such as --format and --json, then print what is left.

FIELDS

A field is a key of the resources, as --format json prints them: a name,
or a dotted path into nested objects (labels.env), with [N] for an element
of a list (disks[0].name) and [] for every element. project_id also finds
projectId, and the other way around.

FILTER EXPRESSIONS

A filter is a strict subset of gcloud's filter expressions. A term is a
field, an operator and a value:

  key:pattern   the value contains pattern, ignoring case, accents and
                HTML tags; abc* is a prefix, *abc a suffix, a*c a whole
                line, and key:* is true when key has a value
  key=value     a line of the value is value, ignoring case and accents
  key!=value    not key=value
  key<value     also <=, > and >=: numbers compare as numbers, other
                values as text; on dates and times (2018-01-15,
                2018-01-15T12:00:00Z), as instants

Terms combine with AND, OR and NOT (in upper case), - before a term (not),
juxtaposition (a b means a AND b, with lower precedence than OR) and
parentheses. AND and OR in the same parentheses need explicit grouping:
(a AND b) OR c. key:(a b) and key=(a,b) match any of the values. Quote
values with spaces or special characters: name:'my project'.

A list value matches when one of its elements does. true and false match
the numbers 1 and 0, and a missing field is null.

Examples:

  --filter 'status=RUNNING AND labels.env=prod'
  --filter 'name:web* OR name:api*'
  --filter 'createTime>=2024-01-01 -labels.team:*'
  --sort-by ~createTime,name --limit 10

Regular expressions (~ and !~), bare words without an operator, transforms
such as name.len(), relative times such as -P2W and other forms gcloud
reads are not supported: they are errors, never a different result.
--sort-by compares the text of the values, as gcloud does, so 10 sorts
before 9.
`
