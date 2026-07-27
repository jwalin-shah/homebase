module Reducer {

  datatype Phase = Active | Recovering | Concluded

  datatype AttemptState = AttemptState(
    phase: Phase,
    recoveryDispatches: int,
    dispatchedEffects: set<string>,
    processedCmdKeys: set<string>,
    version: int
  )

  datatype Command = 
    | CommandProposeRecovery(attemptID: string, idempotencyKey: string, version: int)
    | CommandConclude(attemptID: string)
    | CommandUnknown(attemptID: string)

  datatype Event =
    | EventRecoveryDispatched(attemptID: string, effectID: string, ordinal: int, idempotencyKey: string)
    | EventRecoveryRejected(attemptID: string, reason: string)
    | EventConcluded(attemptID: string)

  datatype EffectIntent = EffectIntent(
    attemptID: string,
    effectID: string,
    ordinal: int
  )

  datatype DecisionStatus = Accepted | Rejected | NoOp

  datatype Decision = Decision(
    status: DecisionStatus,
    events: seq<Event>,
    effects: seq<EffectIntent>
  )

  function Decide(state: AttemptState, cmd: Command): Decision
  {
    match cmd
    case CommandConclude(attemptID) =>
      if state.phase == Concluded then
        Decision(NoOp, [], [])
      else
        Decision(Accepted, [EventConcluded(attemptID)], [])
        
    case CommandUnknown(attemptID) =>
      Decision(Rejected, [], [])
        
    case CommandProposeRecovery(attemptID, idempotencyKey, version) =>
      if version > 0 && version < state.version then
        Decision(Rejected, [EventRecoveryRejected(attemptID, "stale command")], [])
      else if state.phase == Concluded then
        Decision(Rejected, [EventRecoveryRejected(attemptID, "attempt already concluded")], [])
      else if idempotencyKey in state.processedCmdKeys then
        Decision(NoOp, [], [])
      else if state.recoveryDispatches >= 2 then
        Decision(Rejected, [EventRecoveryRejected(attemptID, "recovery limit reached")], [])
      else
        // Generate a pseudo-effectID based on the attemptID and ordinal
        // In reality this might be passed in or hashed, but for simplicity here we just use idempotencyKey
        var effectID := attemptID + "-effect-" + idempotencyKey;
        if effectID in state.dispatchedEffects then
          Decision(Rejected, [EventRecoveryRejected(attemptID, "effect collision")], [])
        else
          Decision(
            Accepted, 
            [EventRecoveryDispatched(attemptID, effectID, state.recoveryDispatches + 1, idempotencyKey)], 
            [EffectIntent(attemptID, effectID, state.recoveryDispatches + 1)]
          )
  }

  function Apply(state: AttemptState, event: Event): AttemptState
  {
    match event
    case EventConcluded(_) =>
      AttemptState(
        Concluded,
        state.recoveryDispatches,
        state.dispatchedEffects,
        state.processedCmdKeys,
        state.version
      )
    case EventRecoveryDispatched(_, effectID, _, idempotencyKey) =>
      AttemptState(
        Recovering,
        state.recoveryDispatches + 1,
        state.dispatchedEffects + {effectID},
        state.processedCmdKeys + {idempotencyKey},
        state.version
      )
    case EventRecoveryRejected(_, _) =>
      state
  }

  function ApplyBatch(state: AttemptState, events: seq<Event>): AttemptState
    decreases |events|
  {
    if |events| == 0 then state
    else ApplyBatch(Apply(state, events[0]), events[1..])
  }

  // Properties

  lemma BoundNeverExceeds(state: AttemptState, cmd: Command)
    requires state.recoveryDispatches <= 2
    ensures ApplyBatch(state, Decide(state, cmd).events).recoveryDispatches <= 2
  {
    // Proved automatically
  }

  lemma TerminalEmitsNoEffects(state: AttemptState, cmd: Command)
    requires state.phase == Concluded
    ensures |Decide(state, cmd).effects| == 0
  {
    // Proved automatically
  }

  lemma RejectedOrNoOpChangesNothing(state: AttemptState, cmd: Command)
    ensures Decide(state, cmd).status == Rejected || Decide(state, cmd).status == NoOp ==>
            ApplyBatch(state, Decide(state, cmd).events) == state
  {
    // Proved automatically
  }

  lemma UniqueEffectIDs(state: AttemptState, cmd: Command)
    requires forall e :: e in state.dispatchedEffects ==> true
    ensures forall e :: e in ApplyBatch(state, Decide(state, cmd).events).dispatchedEffects ==> 
            (e in state.dispatchedEffects || (Decide(state, cmd).status == Accepted && cmd.CommandProposeRecovery? && e == cmd.attemptID + "-effect-" + cmd.idempotencyKey && !(e in state.dispatchedEffects)))
  {
    // Proved automatically
  }
}
