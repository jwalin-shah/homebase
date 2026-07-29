module RecordBoundary {
  datatype Kind = Decision | Contract | CapabilityGrant | Observation | Proposal | Proof | VerificationReceipt | Other
  datatype Authority = HumanDecision | AuthoritativeFact | VerifiedEvidence | WorkerObservation | AgentProposal | UntrustedText
  datatype Role = Captain | Portfolio | HomeBase | Bridge | Worker | Agent | Verifier | Axioms | Trajectory | OtherRole

  datatype Record = Record(id: string)

  function OwnerRole(kind: Kind, role: Role): bool {
    match kind {
      case Decision => role == Captain || role == Portfolio || role == HomeBase
      case Contract => role == Captain || role == Portfolio || role == HomeBase
      case CapabilityGrant => role == Bridge
      case Observation => role == Worker || role == Agent
      case Proposal => role == Worker || role == Agent
      case Proof => role == Verifier || role == Axioms
      case VerificationReceipt => role == Verifier
      case Other => true
    }
  }

  function ExpectedAuthority(kind: Kind): string
  {
    match kind {
      case Decision => "HumanDecision"
      case Contract => "HumanDecision"
      case CapabilityGrant => "AuthoritativeFact"
      case Observation => "WorkerObservation"
      case Proposal => "AgentProposal"
      case Proof => "VerifiedEvidence"
      case VerificationReceipt => "VerifiedEvidence"
      case Other => ""
    }
  }

  function AuthorityName(authority: Authority): string
  {
    match authority {
      case HumanDecision => "HumanDecision"
      case AuthoritativeFact => "AuthoritativeFact"
      case VerifiedEvidence => "VerifiedEvidence"
      case WorkerObservation => "WorkerObservation"
      case AgentProposal => "AgentProposal"
      case UntrustedText => "UntrustedText"
    }
  }

  function Admits(kind: Kind, authority: Authority, role: Role): bool {
    OwnerRole(kind, role) && (ExpectedAuthority(kind) == "" || ExpectedAuthority(kind) == AuthorityName(authority))
  }

  predicate ValidReceipt(sourceRole: Role, sourceID: string, verifierID: string, status: string, treeDigest: string, subjectDigest: string)
  {
    sourceRole == Verifier && sourceID == verifierID && status == "verified" && treeDigest == subjectDigest
  }

  predicate UniqueIDs(history: seq<Record>)
  {
    forall i, j :: 0 <= i < |history| && 0 <= j < |history| && i != j ==> history[i].id != history[j].id
  }

  method CheckAdmission(kind: Kind, authority: Authority, role: Role) returns (admitted: bool)
    ensures admitted == Admits(kind, authority, role)
  {
    admitted := OwnerRole(kind, role);
    var expected := ExpectedAuthority(kind);
	if expected != "" {
	    admitted := admitted && expected == AuthorityName(authority);
    }
  }

  lemma AdmittedRecordsHaveOwnerAndAuthority(kind: Kind, authority: Authority, role: Role)
    ensures Admits(kind, authority, role) ==> OwnerRole(kind, role)
    ensures Admits(kind, authority, role) ==> ExpectedAuthority(kind) == "" || ExpectedAuthority(kind) == AuthorityName(authority)
  {
  }

  lemma ValidReceiptBindsIdentityAndTree(sourceRole: Role, sourceID: string, verifierID: string, status: string, treeDigest: string, subjectDigest: string)
    ensures ValidReceipt(sourceRole, sourceID, verifierID, status, treeDigest, subjectDigest) ==> sourceRole == Verifier
    ensures ValidReceipt(sourceRole, sourceID, verifierID, status, treeDigest, subjectDigest) ==> sourceID == verifierID
    ensures ValidReceipt(sourceRole, sourceID, verifierID, status, treeDigest, subjectDigest) ==> status == "verified"
    ensures ValidReceipt(sourceRole, sourceID, verifierID, status, treeDigest, subjectDigest) ==> treeDigest == subjectDigest
  {
  }

  lemma AppendPreservesUniqueIDs(history: seq<Record>, next: Record)
    requires UniqueIDs(history)
    requires forall i :: 0 <= i < |history| ==> history[i].id != next.id
    ensures UniqueIDs(history + [next])
  {
    forall i, j | 0 <= i < |history + [next]| && 0 <= j < |history + [next]| && i != j
      ensures (history + [next])[i].id != (history + [next])[j].id
    {
    }
  }
}
