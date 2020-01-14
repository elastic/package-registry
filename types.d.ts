// this file was automatically generated, DO NOT EDIT
// classes
// struct2ts:github.com/elastic/package-registry/util.PackageRequirementKibana
interface PackageRequirementKibana {
	versions?: string;
}

// struct2ts:github.com/elastic/package-registry/util.PackageRequirement
interface PackageRequirement {
	kibana: PackageRequirementKibana;
}

// struct2ts:github.com/elastic/package-registry/util.PackageImage
interface PackageImage {
	src?: string;
	title?: string;
	size?: string;
	type?: string;
}

// struct2ts:github.com/elastic/package-registry/util.PackageDataSet
interface PackageDataSet {
	title: string;
	name: string;
	release: string;
	type: string;
	ingest_pipeline?: string;
	vars?: { [key: string]: any }[];
	package: string;
}

// struct2ts:github.com/elastic/package-registry/util.Package
interface Package {
	name: string;
	title?: string | null;
	version: string;
	readme?: string | null;
	description: string;
	type: string;
	categories: string[];
	requirement: PackageRequirement;
	screenshots?: PackageImage[];
	icons?: PackageImage[];
	assets?: string[];
	internal?: boolean;
	format_version: string;
	datasets?: PackageDataSet[];
	download: string;
	path: string;
}

// exports
export {
	PackageRequirementKibana,
	PackageRequirement,
	PackageImage,
	PackageDataSet,
	Package,
};