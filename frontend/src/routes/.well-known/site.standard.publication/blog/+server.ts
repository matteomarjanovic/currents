// The publication record's URL is https://currents.is/blog, so Standard.site verification
// requires this path-specific endpoint in addition to the root discovery endpoint.
// It cannot be prerendered: the root endpoint is a file at the parent path, and static
// filesystems cannot also create a directory with that same name.
export const prerender = false;

export { GET } from '../+server';
