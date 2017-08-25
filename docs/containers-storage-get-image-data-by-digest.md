## containers-storage-get-image-data-by-digest 1 "August 2017"

## NAME
containers-storage get-image-data-by-digest - Retrieve the digest of a lookaside data item

## SYNOPSIS
**containers-storage** **get-image-data-by-digest** *digest*

## DESCRIPTION
Prints a list of pairs of image IDs and the names of data items which they
contain which match the specified digest.  The digest should be in the form
*sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef*.

## EXAMPLE
**containers-storage get-image-data-by-digest sha256:0123456789abcdef...**

## SEE ALSO
containers-storage-get-image-data(1)
containers-storage-get-image-data-size(1)
containers-storage-get-image-data-digest(1)
containers-storage-list-image-data(1)
containers-storage-set-image-data(1)
