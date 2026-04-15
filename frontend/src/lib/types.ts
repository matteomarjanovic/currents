export interface CollectionView {
	uri: string;
	cid?: string;
	name: string;
	description?: string;
	saveCount?: number;
	previewImages?: string[];
	createdAt?: string;
	viewer?: { starred?: boolean };
}

export interface ActorProfileView {
	did: string;
	handle: string;
	displayName?: string;
	description?: string;
	pronouns?: string;
	website?: string;
	avatar?: string;
	banner?: string;
	createdAt?: string;
}

export interface SaveView {
	uri: string;
	blobCid: string;
	author: {
		did: string;
		handle: string;
		displayName?: string;
		avatar?: string;
	};
	imageUrl: string;
	text?: string;
	originUrl?: string;
	attribution?: { url?: string; license?: string; credit?: string };
	resaveOf?: { uri: string; cid: string };
	createdAt: string;
	viewer?: { saves?: { collectionUri: string; saveUri: string }[] };
	width?: number;
	height?: number;
	dominantColor?: string;
}
