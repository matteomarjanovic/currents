now let's create crud endpoints for the save and collection entities. all the crud apis will just perform crud operations using the com.atproto.repo.* methods (createRecord, putRecord, deleteRecord). you can find the lexicons for save and collection in the @..\..\..\Documenti\robe\cu-so\lexicons\ folder. in particular, you will just have to create endpoints that use comatproto.RepoListRecords, comatproto.RepoGetRecord, comatproto.RepoCreateRecord, comatproto.RepoPutRecord and comatproto.RepoDeleteRecord from the indigo sdk. Note that you'll need to blank import the bsky and util libraries if you aren't using them directly, due to the way Go imports work:

_ "github.com/bluesky-social/indigo/api/bsky"
_ "github.com/bluesky-social/indigo/lex/util".

the api routes should be something like:
POST /collections - create a new collection
GET /collections/{id} - get a collection by id
GET /collections - list all collections for the user
PUT /collections/{id} - update a collection by id
DELETE /collections/{id} - delete a collection by id
POST /saves - create a new save
GET /saves/{id} - get a save by id
GET /saves - list all saves for the user
PUT /saves/{id} - update a save by id
DELETE /saves/{id} - delete a save by id

for the record creation of the save entity, you will need to upload the blob to the user's PDS first, get the blob CID, and then include that in the pds_blob_cid field of the save record. for now, let's keep it simple and not worry about image embeddings or visual identity, we'll add that later. just focus on getting the basic crud operations working for collections and saves. add these operations in the example html pages that we have in the appview/templates folder, so we can test them with simple forms.

 one final note from the at protocol documentation: Read-after-Write
Depending on how an AT app frontend is designed, a user may take some action (such as updating their profile), rapidly refresh the view, and find that their recent change is not immediately reflected in the updated response.

This is because services such as the App View do not have transactional writes from a user's PDS. Therefore the views that they calculate are eventually consistent. In other words, a user may have created a record on their PDS that is not yet reflected in the API responses provided by the App View.

Because all requests from the application are sent to the user's PDS, the PDS is in a position to smooth over this behavior. Our PDS distribution provides some basic read-after-write behaviors by looking at response headers from the App View, determining if there are any new records that are not in the response, and modifying the response to reflect those new records.

The App View communicates the current state of its indices by setting the Atproto-Repo-Rev response header. This is set by the rev of the most recent commit that's been indexed from the requesting user's repository. If the PDS sees this header on a response, it will search for all records that it has locally that are more recent than the provided rev and determine if they affect the App View's response.

This read-after-write behavior only applies to records from the user making the request. Records from other users that happen to be on the same PDS will not affect the requesting user's response.
