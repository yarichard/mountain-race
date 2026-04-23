"use client";

import { useEffect } from "react";
import { MapContainer, TileLayer, Marker, Popup, Polyline, useMap } from "react-leaflet";
import "leaflet/dist/leaflet.css";
import L from "leaflet";

const icon = L.icon({
  iconUrl: "/mountain-climb.svg",
  iconSize: [36, 36],
  iconAnchor: [18, 36],
  popupAnchor: [0, -36],
});

function FitBounds({ track }: { track: [number, number][] }) {
  const map = useMap();
  useEffect(() => {
    if (track.length > 1) {
      map.fitBounds(L.latLngBounds(track), { padding: [16, 16] });
    }
  }, [map, track]);
  return null;
}

interface Props {
  lat: number;
  lon: number;
  track?: [number, number][];
}

export default function MapView({ lat, lon, track }: Props) {
  const hasTrack = track && track.length > 1;
  return (
    <MapContainer
      center={[lat, lon]}
      zoom={13}
      style={{ width: "100%", height: "100%" }}
      scrollWheelZoom={false}
    >
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />
      <Marker position={[lat, lon]} icon={icon}>
        <Popup>Point de départ</Popup>
      </Marker>
      {hasTrack && (
        <>
          <Polyline positions={track} color="#1F2782" weight={3} opacity={0.85} />
          <FitBounds track={track} />
        </>
      )}
    </MapContainer>
  );
}
