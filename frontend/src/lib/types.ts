export interface CollectionView {
	uri: string;
	name: string;
	saveCount?: number;
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
