= JUnit Merger =

This tool helps merging test results from parallel and serial executions to provider a common overview.
The test sets are mutually exclusive. As a result naively merging the junit files leads to an insane amount of
skipped tests. One location where this occurs is in Deck from prow.

The tool will doe the following things which need to extra pointed out:
 * Skipped tests are deduplicated and their execution time gets merged
 * The tool fails if some tests are run in more than one provided junit file
