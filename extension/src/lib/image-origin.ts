export interface StrongRef {
	uri: string;
	cid: string;
}

// App adapters identify candidate records in the page. The worker verifies
// ownership and resolves a record CID before adding provenance to the save.
export interface BlueskyImageOriginHint {
	app: 'bluesky';
	client: string;
	did: string;
	blobCid: string;
	posts: string[];
}

export interface GrainImageOriginHint {
	app: 'grain';
	did: string;
	blobCid: string;
	galleries: string[];
}

export interface PcktImageOriginHint {
	app: 'pckt';
	imageUrl: string;
	pages: string[];
	snapshot?: ImageOrigin;
}

export interface OffprintImageOriginHint {
	app: 'offprint';
	imageUrl: string;
	pages: string[];
	snapshot?: ImageOrigin;
}

export interface LeafletImageOriginHint {
	app: 'leaflet';
	did: string;
	blobCid: string;
	uri?: string;
	pages: string[];
}

export type ImageOriginHint =
	| BlueskyImageOriginHint
	| GrainImageOriginHint
	| PcktImageOriginHint
	| OffprintImageOriginHint
	| LeafletImageOriginHint;

export interface ImageOrigin {
	originAtprotoRef: StrongRef;
	originUrl: string;
	// Only the matched image's dedicated accessibility text, never a caption.
	alt?: string;
}
