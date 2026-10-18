package graphql

// ThreadsSlideDeltasQuery is the prevalidated doc ID threads.com sends in the lightspeed sync
// params (the counterpart of IGDSlideDeltaProcessorQuery). threads.com syncs database 231.
const ThreadsSlideDeltasQuery = "28611760365183156"

// ThreadsTypingIndicatorSubscription is the doc ID of the threads.com typing subscription
// (BarcelonaMessageListTypingIndicatorFooterSubscription) on the streamcontroller socket.
const ThreadsTypingIndicatorSubscription = "25253734247645895"

// ThreadsSyncDatabase is the iris database threads.com subscribes to over DGW lightspeed.
const ThreadsSyncDatabase = 231

// threadsDocs are the threads.com messaging operations, read from the web bundles
// (modules "<Op>_threadsRelayOperation"). Doc IDs rotate with web releases.
var threadsDocs = map[string]GraphQLDoc{
	"BarcelonaMessageListContainerQuery":                {DocID: "29328923396704798", FriendlyName: "BarcelonaMessageListContainerQuery"},
	"BarcelonaMessageListPaginationQuery":               {DocID: "28669506596011480", FriendlyName: "BarcelonaMessageListPaginationQuery"},
	"BarcelonaMessagesSetup_MailboxQuery":               {DocID: "27814147468201832", FriendlyName: "BarcelonaMessagesSetup_MailboxQuery"},
	"BarcelonaMessagesThreadListPaginationQuery":        {DocID: "27218974577800825", FriendlyName: "BarcelonaMessagesThreadListPaginationQuery"},
	"BarcelonaNewThreadMessageListContainerQuery":       {DocID: "28723860420627939", FriendlyName: "BarcelonaNewThreadMessageListContainerQuery"},
	"WmiSlideAsyncFetchAndInsertThreadQuery":            {DocID: "29255577000740698", FriendlyName: "WmiSlideAsyncFetchAndInsertThreadQuery"},
	"WmiNormalizeSlideMessageQuery":                     {DocID: "28478867261785107", FriendlyName: "WmiNormalizeSlideMessageQuery"},
	"SlideUQPPDeltasQuery":                              {DocID: "28611760365183156", FriendlyName: "SlideUQPPDeltasQuery"},
	"WmiSlideResnapshotHandlerQuery":                    {DocID: "28489149227434804", FriendlyName: "WmiSlideResnapshotHandlerQuery"},
	"BarcelonaNewThreadContactSearchResultsQuery":       {DocID: "37960114713633915", FriendlyName: "BarcelonaNewThreadContactSearchResultsQuery"},
	"BarcelonaAddMembersDialogThreadQuery":              {DocID: "29279209118352987", FriendlyName: "BarcelonaAddMembersDialogThreadQuery"},
	"useBarcelonaNavigateToUserThread_EligibilityQuery": {DocID: "27206623385607896", FriendlyName: "useBarcelonaNavigateToUserThread_EligibilityQuery"},
	"WmiSlideTextMessageMutation":                       {DocID: "27966697313006171", FriendlyName: "WmiSlideTextMessageMutation"},
	"WmiSlideMediaMessageMutation":                      {DocID: "29545984378321535", FriendlyName: "WmiSlideMediaMessageMutation"},
	"WmiSlideMultiMediaMessageMutation":                 {DocID: "28865105503142793", FriendlyName: "WmiSlideMultiMediaMessageMutation"},
	"WmiSlideEditMessageMutation":                       {DocID: "26393953693557273", FriendlyName: "WmiSlideEditMessageMutation"},
	"WmiSlideUnsendForEveryoneMessageMutation":          {DocID: "32965285823085012", FriendlyName: "WmiSlideUnsendForEveryoneMessageMutation"},
	"WmiSlideUnsendForMeMessageMutation":                {DocID: "33163241023290129", FriendlyName: "WmiSlideUnsendForMeMessageMutation"},
	"WmiSlideSetReactionMutation":                       {DocID: "27404150755848498", FriendlyName: "WmiSlideSetReactionMutation"},
	"WmiSlideRemoveReactionMutation":                    {DocID: "28156325447327496", FriendlyName: "WmiSlideRemoveReactionMutation"},
	"WmiSlidePublishTypingIndicatorMutation":            {DocID: "25329879396664807", FriendlyName: "WmiSlidePublishTypingIndicatorMutation"},
	"WmiSlideThreadMutationsCreate1To1Mutation":         {DocID: "29491551170533903", FriendlyName: "WmiSlideThreadMutationsCreate1To1Mutation"},
	"WmiSlideThreadMutationsCreateGroupMutation":        {DocID: "39257347767244168", FriendlyName: "WmiSlideThreadMutationsCreateGroupMutation"},
	"WmiSlideThreadMutationsDeleteMutation":             {DocID: "25363774693243351", FriendlyName: "WmiSlideThreadMutationsDeleteMutation"},
	"WmiSlideThreadMutationsMarkReadMutation":           {DocID: "25220561634307920", FriendlyName: "WmiSlideThreadMutationsMarkReadMutation"},
	"WmiSlideThreadMutationsMuteMutation":               {DocID: "26028135133464433", FriendlyName: "WmiSlideThreadMutationsMuteMutation"},
	"useBarcelonaMessagesThreadAcceptActionMutation":    {DocID: "37545414765049339", FriendlyName: "useBarcelonaMessagesThreadAcceptActionMutation"},
	"WmiSlideChangeThreadNameMutation":                  {DocID: "25736621219362855", FriendlyName: "WmiSlideChangeThreadNameMutation"},
	"WmiSlideChangeThreadImageMutation":                 {DocID: "25926662796985872", FriendlyName: "WmiSlideChangeThreadImageMutation"},
	"WmiSlideAddParticipantsMutation":                   {DocID: "29220017877598077", FriendlyName: "WmiSlideAddParticipantsMutation"},
	"WmiSlideRemoveParticipantMutation":                 {DocID: "25897531556594905", FriendlyName: "WmiSlideRemoveParticipantMutation"},
	"WmiSlideAdminManagementAddAdminsMutation":          {DocID: "38166879752925961", FriendlyName: "WmiSlideAdminManagementAddAdminsMutation"},
	"WmiSlideAdminManagementRemoveAdminsMutation":       {DocID: "27988874430733698", FriendlyName: "WmiSlideAdminManagementRemoveAdminsMutation"},
}

func init() {
	for name, doc := range threadsDocs {
		GraphQLDocs[name] = doc
	}
}
