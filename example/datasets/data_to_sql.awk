# To get the INSERT statements in popularize_table.sql, please apply
# this script to the dataset downloaded from
# https://www.kaggle.com/blastchar/telco-customer-churn.  Then,
# manually remove the few lines that contains " ".  Before applying
# this AWK program, please convert the downloaded CSV file from DOS
# format into UNIX format, and remove the first line of metadata.
BEGIN {
    FS = ",";
}

{
    printf("INSERT INTO %s VALUES(", table);
    for (i = 1; i <= NF; i++) {
	printf("\"%s\"", $i);
	if (i < NF) {
	    printf(",");
	}
    }
    printf(");\n");
}
