## containers-storage-get-container-data-by-digest 1 "August 2017"

## NAME
containers-storage get-container-data-by-digest - Retrieve the digest of a lookaside data item

## SYNOPSIS
**containers-storage** **get-container-data-by-digest** *digest*

## DESCRIPTION
Prints a list of pairs of container IDs and the names of data items which they
contain which match the specified digest.  The digest should be in the form
*sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef*.

## EXAMPLE
**containers-storage get-container-data-by-digest sha256:0123456789abcdef...**

## SEE ALSO
containers-storage-get-container-data(1)
containers-storage-get-container-data-size(1)
containers-storage-get-container-data-digest(1)
containers-storage-list-container-data(1)
containers-storage-set-container-data(1)
