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
	createdAt: string;
	viewer?: { resaved?: boolean };
	width?: number;
	height?: number;
	colors?: { hex: string; fraction: number }[];
}
