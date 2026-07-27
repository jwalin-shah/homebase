module Reducer {
  datatype Event =
    | EventRecoveryDispatched(effectID: string, ordinal: int)
    | EventRecoveryRejected(reason: string)

  datatype AttemptState = AttemptState(
    recoveryDispatches: int
  )

  function Apply(state: AttemptState, event: Event): AttemptState
  {
    match event
    case EventRecoveryDispatched(_, _) => AttemptState(state.recoveryDispatches + 1)
    case EventRecoveryRejected(_) => state
  }

  lemma BoundNeverExceeds(state: AttemptState, event: Event)
    requires state.recoveryDispatches <= 2
    requires event.EventRecoveryDispatched? ==> state.recoveryDispatches < 2
    ensures Apply(state, event).recoveryDispatches <= 2
  {
  }
}
