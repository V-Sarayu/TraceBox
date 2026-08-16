import { useEffect, useState, useCallback } from 'react';
import ReactFlow, { Background, Controls, MarkerType } from 'reactflow';
import 'reactflow/dist/style.css';

const API_BASE = 'http://localhost:8080';

function StatusPill({ status }) {
  const color = status >= 500 ? '#e5484d' : status >= 400 ? '#f5a623' : '#30a46c';
  return (
    <span style={{
      background: color, color: '#fff', padding: '2px 8px',
      borderRadius: 12, fontSize: 12, fontWeight: 600
    }}>{status}</span>
  );
}

function HistoryView() {
  const [requests, setRequests] = useState([]);
  const [selected, setSelected] = useState([]);
  const [diffResult, setDiffResult] = useState(null);
  const [replayResult, setReplayResult] = useState(null);
  const [loading, setLoading] = useState(true);

  const fetchRequests = useCallback(() => {
    fetch(`${API_BASE}/api/requests`)
      .then(r => r.json())
      .then(data => { setRequests(data || []); setLoading(false); })
      .catch(() => setLoading(false));
  }, []);

  useEffect(() => {
    fetchRequests();
    const interval = setInterval(fetchRequests, 4000); // poll for near-live updates
    return () => clearInterval(interval);
  }, [fetchRequests]);

  const toggleSelect = (id) => {
    setSelected(prev => {
      if (prev.includes(id)) return prev.filter(x => x !== id);
      if (prev.length >= 2) return [prev[1], id];
      return [...prev, id];
    });
  };

  const runDiff = () => {
    if (selected.length !== 2) return;
    fetch(`${API_BASE}/api/diff?a=${selected[0]}&b=${selected[1]}`)
      .then(r => r.json())
      .then(setDiffResult);
  };

  const runReplay = (id) => {
    fetch(`${API_BASE}/api/replay?id=${id}`)
      .then(r => r.json())
      .then(setReplayResult);
  };

  if (loading) return <p style={{ padding: 20 }}>Loading requests…</p>;

  return (
    <div style={{ padding: 20, fontFamily: 'system-ui, sans-serif' }}>
      <div style={{ display: 'flex', gap: 12, alignItems: 'center', marginBottom: 16 }}>
        <h2 style={{ margin: 0 }}>Request History</h2>
        <span style={{ color: '#888', fontSize: 13 }}>
          Select two rows to diff · {selected.length}/2 selected
        </span>
        <button onClick={runDiff} disabled={selected.length !== 2}
          style={{ padding: '6px 14px', borderRadius: 6, cursor: 'pointer' }}>
          Diff Selected
        </button>
      </div>

      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
        <thead>
          <tr style={{ textAlign: 'left', borderBottom: '2px solid #eee' }}>
            <th></th>
            <th style={{ padding: 8 }}>Method</th>
            <th style={{ padding: 8 }}>Path</th>
            <th style={{ padding: 8 }}>Service</th>
            <th style={{ padding: 8 }}>Status</th>
            <th style={{ padding: 8 }}>Duration</th>
            <th style={{ padding: 8 }}>Trace ID</th>
            <th style={{ padding: 8 }}></th>
          </tr>
        </thead>
        <tbody>
          {requests.map(r => (
            <tr key={r.ID} style={{ borderBottom: '1px solid #f0f0f0' }}>
              <td style={{ padding: 8 }}>
                <input type="checkbox" checked={selected.includes(r.ID)}
                  onChange={() => toggleSelect(r.ID)} />
              </td>
              <td style={{ padding: 8, fontWeight: 600 }}>{r.Method}</td>
              <td style={{ padding: 8 }}>{r.Path}</td>
              <td style={{ padding: 8, color: '#888' }}>{r.TargetService}</td>
              <td style={{ padding: 8 }}><StatusPill status={r.ResponseStatus} /></td>
              <td style={{ padding: 8 }}>{r.DurationMs}ms</td>
              <td style={{ padding: 8, fontFamily: 'monospace', fontSize: 12, color: '#888' }}>
                {r.TraceID?.slice(0, 10)}…
              </td>
              <td style={{ padding: 8 }}>
                <button onClick={() => runReplay(r.ID)}
                  style={{ fontSize: 12, padding: '4px 10px', borderRadius: 6, cursor: 'pointer' }}>
                  Replay
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {replayResult && (
        <div style={{ marginTop: 20, padding: 14, background: '#f7f7f8', borderRadius: 8 }}>
          <strong>Replay result:</strong> original status {replayResult.original_status} → replay status {replayResult.replay_status}
        </div>
      )}

      {diffResult && (
        <div style={{ marginTop: 20, padding: 14, background: '#f7f7f8', borderRadius: 8 }}>
          <h3 style={{ marginTop: 0 }}>Diff Result</h3>
          <p><strong>Status:</strong> {diffResult.status_a} → {diffResult.status_b}{' '}
            {diffResult.status_changed && <span style={{ color: '#e5484d' }}>(changed)</span>}</p>
          {diffResult.body_changed && (
            <div style={{ display: 'flex', gap: 12 }}>
              <pre style={{ flex: 1, background: '#fff', padding: 10, borderRadius: 6, overflow: 'auto', fontSize: 12 }}>
                {diffResult.body_a}
              </pre>
              <pre style={{ flex: 1, background: '#fff', padding: 10, borderRadius: 6, overflow: 'auto', fontSize: 12 }}>
                {diffResult.body_b}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function GraphView() {
  const [nodes, setNodes] = useState([]);
  const [edges, setEdges] = useState([]);

  const fetchGraph = useCallback(() => {
    fetch(`${API_BASE}/api/graph`)
      .then(r => r.json())
      .then(data => {
        const nodeIds = (data.nodes || []).map(n => n.id);
        const laidOut = nodeIds.map((id, i) => ({
          id,
          position: { x: (i % 4) * 220, y: Math.floor(i / 4) * 140 },
          data: { label: id },
          style: {
            padding: 10, borderRadius: 8,
            background: id === 'client' ? '#111' : '#fff',
            color: id === 'client' ? '#fff' : '#111',
            border: '1px solid #ddd', fontSize: 13,
          },
        }));
        const flowEdges = (data.edges || []).map((e, i) => ({
          id: `e${i}`,
          source: e.source,
          target: e.target,
          label: `${e.call_count}x · avg ${e.avg_latency_ms.toFixed(0)}ms`,
          animated: e.is_slow,
          style: { stroke: e.is_slow ? '#e5484d' : '#999', strokeWidth: e.is_slow ? 2.5 : 1.5 },
          labelStyle: { fill: e.is_slow ? '#e5484d' : '#666', fontSize: 11, fontWeight: e.is_slow ? 700 : 400 },
          markerEnd: { type: MarkerType.ArrowClosed },
        }));
        setNodes(laidOut);
        setEdges(flowEdges);
      });
  }, []);

  useEffect(() => {
    fetchGraph();
    const interval = setInterval(fetchGraph, 4000);
    return () => clearInterval(interval);
  }, [fetchGraph]);

  return (
    <div style={{ height: 'calc(100vh - 100px)' }}>
      <ReactFlow nodes={nodes} edges={edges} fitView>
        <Background />
        <Controls />
      </ReactFlow>
      <p style={{ padding: '0 20px', color: '#888', fontSize: 13 }}>
        Red animated edges = flagged as slow (avg latency above threshold)
      </p>
    </div>
  );
}

export default function App() {
  const [tab, setTab] = useState('history');

  return (
    <div>
      <div style={{
        display: 'flex', gap: 8, padding: '14px 20px',
        borderBottom: '1px solid #eee', fontFamily: 'system-ui, sans-serif'
      }}>
        <h1 style={{ fontSize: 18, margin: 0, marginRight: 20 }}>TraceBox</h1>
        <button onClick={() => setTab('history')}
          style={{ fontWeight: tab === 'history' ? 700 : 400, cursor: 'pointer', padding: '4px 10px' }}>
          History & Diff
        </button>
        <button onClick={() => setTab('graph')}
          style={{ fontWeight: tab === 'graph' ? 700 : 400, cursor: 'pointer', padding: '4px 10px' }}>
          Dependency Graph
        </button>
      </div>
      {tab === 'history' ? <HistoryView /> : <GraphView />}
    </div>
  );
}