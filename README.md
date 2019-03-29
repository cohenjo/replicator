# Replicator

Replicator is a go package aimed to test out different options to take change stream off DB engines.
It's inspired by [canal](https://github.com/siddontang/go-mysql#canal) which provides a change stream off MySQL.

Mongo has [Change Streams](https://docs.mongodb.com/manual/changeStreams/#change-streams) - so this should be doable easily.
PG also uses binary logs (WALS) to transfer replication - so that's also on the agenda.