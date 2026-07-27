BEGIN {
	threshold = threshold + 0
	failed = 0
}

NR == 1 && $1 == "mode:" {
	next
}

{
	file = $1
	sub(/:[0-9].*$/, "", file)

	if (file ~ /\.pb\.go$/ ||
	    file ~ /\.pb\.gw\.go$/ ||
	    file ~ /_grpc\.pb\.go$/ ||
	    file ~ /\/mocks\// ||
	    file ~ /_mock\.go$/ ||
	    file ~ /\/database\/mock\.go$/ ||
	    file ~ /\/database\/redis\/mock\.go$/ ||
	    file ~ /\/bench\// ||
	    file ~ /\/testing\//) {
		next
	}

	pkg = file
	sub(/\/[^\/]*$/, "", pkg)
	sub(/^github\.com\/TeneficGames\/podium\/?/, "", pkg)
	if (pkg == "") {
		pkg = "."
	}

	statements = $(NF - 1)
	total[pkg] += statements
	if ($NF > 0) {
		covered[pkg] += statements
	}
}

END {
	for (pkg in total) {
		percentage = 100 * covered[pkg] / total[pkg]
		printf "%-40s %6.2f%%\n", pkg, percentage
		if (covered[pkg] * 100 < threshold * total[pkg]) {
			failed = 1
		}
	}

	if (failed) {
		printf "One or more packages are below the %.2f%% coverage threshold.\n", threshold > "/dev/stderr"
		exit 1
	}
}
