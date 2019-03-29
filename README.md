# Replicator

Replicator is a go package aimed to test out different options to take change stream off DB engines.
It's inspired by [canal](https://github.com/siddontang/go-mysql#canal) which provides a change stream off MySQL.

Mongo has [Change Streams](https://docs.mongodb.com/manual/changeStreams/#change-streams) - so this should be doable easily.
For Kafka I plan to consume [kasper](https://github.com/nmaquet/kasper)
PG also uses binary logs (WALS) to transfer replication - so that's also on the agenda.

Once we got an event for a record change (insert/update/delete) we propogate the change to the registered endpoints.
at first phase we'll do a straight mapping of field names.

we plan to support field mapping, field filtering, and maybe transformations.


