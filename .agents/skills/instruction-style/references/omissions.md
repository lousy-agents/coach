# What this contract deliberately omits

Earlier versions carried ASD-STE100 rules: sentence-length caps, a ban on
unbound qualifiers, a compound-noun limit, a ban on contractions. They are
omitted on purpose, recorded here so a later pass does not restore them as an
oversight.

STE100 serves a reader whose English is limited. That is not this reader. Its
caps cost subordination, and subordination (`so that`, `even though`, `rather
than`) is where causal reasoning lives. Its qualifier ban cannot tell a
degree-qualifier like `fast` from an intent marker like `deliberately`, so
applying it mechanically strips exactly the words in the keep list.

A closed modal set is kept because it fixes misprioritization and costs nothing
structurally. The contract does not name RFC 2119: that standard's mandatory
word is `MUST`, and mandating a `MUST` → `shall` rewrite would fight the house
style of every Copilot-owned and GitHub-scaffolded file it touched. Closing the
set matters; which words close it does not.

EARS is kept, scoped to triggers, faults, and ordered procedures, because it
has no slot for a reason — an author who applies it everywhere strips rationale
as a matter of course.
