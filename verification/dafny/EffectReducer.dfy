module EffectReducer {

  datatype Outcome = Succeeded | FailedRetryable | FailedTerminal | OutcomeUnknown
  datatype EffectPhase = Pending | Claimed | SucceededPhase | FailedRetryablePhase | FailedTerminalPhase | OutcomeUnknownPhase

  datatype EffectState = EffectState(
    effectID: string,
    attemptID: string,
    phase: EffectPhase,
    workerID: string,
    claimEpoch: int,
    leaseUntil: int,
    retries: int
  )

  datatype Command = 
    | CommandClaimEffect(effectID: string, workerID: string, expectedVersion: int, leaseUntil: int, currentTime: int)
    | CommandObserveEffect(effectID: string, workerID: string, claimEpoch: int, outcome: Outcome)
    | CommandRetryEffect(effectID: string)

  datatype Event =
    | EventEffectClaimed(effectID: string, workerID: string, claimEpoch: int, leaseUntil: int)
    | EventEffectObserved(effectID: string, claimEpoch: int, outcome: Outcome)
    | EventEffectRetried(effectID: string)
    | EventEffectTerminalized(effectID: string, reason: string)
    | EventEffectRejected(effectID: string, reason: string)

  datatype DecisionStatus = Accepted | Rejected | NoOp

  datatype Decision = Decision(
    status: DecisionStatus,
    events: seq<Event>
  )

  function Decide(state: EffectState, cmd: Command): Decision
  {
    match cmd
    case CommandClaimEffect(effectID, workerID, expectedVersion, leaseUntil, currentTime) =>
      if state.phase == SucceededPhase || state.phase == FailedTerminalPhase then
        Decision(Rejected, [EventEffectRejected(effectID, "effect already terminal")])
      else if state.phase == Claimed && currentTime < state.leaseUntil then
        Decision(Rejected, [EventEffectRejected(effectID, "effect already claimed and unexpired")])
      else if state.retries >= 3 then
        // Policy bound
        Decision(Rejected, [EventEffectTerminalized(effectID, "retry limit reached")])
      else
        Decision(Accepted, [EventEffectClaimed(effectID, workerID, state.claimEpoch + 1, leaseUntil)])

    case CommandObserveEffect(effectID, workerID, claimEpoch, outcome) =>
      if claimEpoch < state.claimEpoch then
        Decision(Rejected, [EventEffectRejected(effectID, "stale claim epoch")])
      else if state.phase != Claimed then
        Decision(Rejected, [EventEffectRejected(effectID, "effect not claimed")])
      else
        Decision(Accepted, [EventEffectObserved(effectID, claimEpoch, outcome)])

    case CommandRetryEffect(effectID) =>
      if state.phase == SucceededPhase || state.phase == FailedTerminalPhase then
        Decision(Rejected, [EventEffectRejected(effectID, "effect already terminal")])
      else if state.phase == FailedRetryablePhase || state.phase == OutcomeUnknownPhase then
        Decision(Accepted, [EventEffectRetried(effectID)])
      else
        Decision(Rejected, [EventEffectRejected(effectID, "not in retryable state")])
  }

  function Apply(state: EffectState, event: Event): EffectState
  {
    match event
    case EventEffectClaimed(_, workerID, claimEpoch, leaseUntil) =>
      EffectState(
        state.effectID,
        state.attemptID,
        Claimed,
        workerID,
        claimEpoch,
        leaseUntil,
        state.retries + 1
      )
    case EventEffectObserved(_, _, outcome) =>
      var newPhase := match outcome {
        case Succeeded => SucceededPhase
        case FailedRetryable => FailedRetryablePhase
        case FailedTerminal => FailedTerminalPhase
        case OutcomeUnknown => OutcomeUnknownPhase
      };
      EffectState(
        state.effectID,
        state.attemptID,
        newPhase,
        state.workerID,
        state.claimEpoch,
        state.leaseUntil,
        state.retries
      )
    case EventEffectRetried(_) =>
      EffectState(
        state.effectID,
        state.attemptID,
        Pending,
        "",
        state.claimEpoch,
        0,
        state.retries
      )
    case EventEffectTerminalized(_, _) =>
      EffectState(
        state.effectID,
        state.attemptID,
        FailedTerminalPhase,
        state.workerID,
        state.claimEpoch,
        state.leaseUntil,
        state.retries
      )
    case EventEffectRejected(_, _) =>
      state
  }

  function ApplyBatch(state: EffectState, events: seq<Event>): EffectState
    decreases |events|
  {
    if |events| == 0 then state
    else ApplyBatch(Apply(state, events[0]), events[1..])
  }

  // Properties

  lemma TerminalCannotReturnToPending(state: EffectState, cmd: Command)
    requires state.phase == SucceededPhase || state.phase == FailedTerminalPhase
    ensures ApplyBatch(state, Decide(state, cmd).events).phase == SucceededPhase || ApplyBatch(state, Decide(state, cmd).events).phase == FailedTerminalPhase
  {
  }

  lemma UnexpiredClaimPreventsNewClaim(state: EffectState, effectID: string, workerID: string, expectedVersion: int, leaseUntil: int, currentTime: int)
    requires state.phase == Claimed && currentTime < state.leaseUntil
    ensures Decide(state, CommandClaimEffect(effectID, workerID, expectedVersion, leaseUntil, currentTime)).status == Rejected
  {
  }

  lemma RetriesBounded(state: EffectState, effectID: string, workerID: string, expectedVersion: int, leaseUntil: int, currentTime: int)
    requires state.retries >= 3
    ensures Decide(state, CommandClaimEffect(effectID, workerID, expectedVersion, leaseUntil, currentTime)).status == Rejected
  {
  }

  lemma StaleWorkerCannotObserve(state: EffectState, effectID: string, workerID: string, claimEpoch: int, outcome: Outcome)
    requires claimEpoch < state.claimEpoch
    ensures Decide(state, CommandObserveEffect(effectID, workerID, claimEpoch, outcome)).status == Rejected
  {
  }

  lemma DuplicateObservationRejected(state: EffectState, effectID: string, workerID: string, claimEpoch: int, outcome: Outcome)
    requires state.phase != Claimed
    ensures Decide(state, CommandObserveEffect(effectID, workerID, claimEpoch, outcome)).status == Rejected
  {
  }

}
